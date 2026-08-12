package pitcoverage

import (
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/exchange_meta"
	"github.com/david22573/ak-historian/internal/lifecycle"
	"github.com/david22573/ak-historian/internal/universe"
)

func (b *Builder) evaluateCoverage(report *Report, lm *lifecycle.Manifest, um *universe.Manifest, sm *exchange_meta.SnapshotManifest) error {
	overallStatus := StatusPitEligible
	// The legacy PIT report remains useful for research diagnostics, but strict
	// promotion is reserved for the canonical research-identity contract.
	overallPromo := PromoExploratoryOnly
	overallRisk := conservativePITRisk(um.SurvivorshipBiasRisk)

	for _, uSym := range um.Symbols {
		if !uSym.ActiveDuringWindow {
			continue
		}

		lSym, ok := findLifecycleSymbol(lm, uSym.Symbol)
		if !ok {
			symEntry := SymbolEntry{
				Symbol:                   uSym.Symbol,
				LifecycleStatus:          "MISSING",
				EvidenceLevel:            "MISSING",
				PointInTimeStatus:        SymStatusMissingLifecycle,
				PromotionBlockingReasons: []string{"MISSING_LIFECYCLE_MANIFEST_OR_ENTRY"},
			}
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolMissingLifecycle,
				Severity:        "ERROR",
				Reason:          "Symbol active in universe but missing from lifecycle manifest",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    uSym.Symbol,
				BlocksPromotion: true,
				RecommendedFix:  "Ensure lifecycle manifest includes this symbol",
			})
			report.Symbols = append(report.Symbols, symEntry)
			report.Validation.IsValid = false
			overallStatus = StatusPitNotEligible
			overallPromo = PromoBlockStrict
			overallRisk = RiskHigh
			continue
		}

		symEntry := SymbolEntry{
			Symbol:                     lSym.Symbol,
			LifecycleStatus:            lSym.Status,
			EvidenceLevel:              lSym.EvidenceLevel,
			TrustLevelSummary:          make(map[string]int),
			ListedAtUTC:                copyStringPtr(lSym.ListedAtUTC),
			DelistedAtUTC:              copyStringPtr(lSym.DelistedAtUTC),
			FirstSeenUTC:               copyStringPtr(lSym.FirstSeenUTC),
			LastSeenUTC:                copyStringPtr(lSym.LastSeenUTC),
			ActiveDuringResearchWindow: true,
			PromotionBlockingReasons:   []string{},
		}

		for _, src := range lSym.Sources {
			symEntry.TrustLevelSummary[src.Confidence]++
			if !isTrustedPITSource(src.Confidence) {
				symEntry.UnverifiedSnapshotCount++
			}
			if isKnownPITTimestamp(src.ObservedAtUTC) {
				symEntry.ObservedSnapshotCount++
			} else {
				symEntry.MissingObservedTimeCount++
			}
			symEntry.SnapshotPresenceCount++
		}

		// Evaluate point-in-time status for the symbol
		if lSym.EvidenceLevel == lifecycle.EvidenceUserProvidedUnverified {
			symEntry.PointInTimeStatus = SymStatusUnverifiedOnly
			symEntry.PromotionBlockingReasons = append(symEntry.PromotionBlockingReasons, "UNVERIFIED_EVIDENCE")
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolUnverifiedOnly,
				Severity:        "ERROR",
				Reason:          "Lifecycle evidence is user-provided or otherwise unverified",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    lSym.Symbol,
				BlocksPromotion: true,
				RecommendedFix:  "Provide source-observed official exchange archive evidence",
			})
			overallStatus = StatusPitNotEligible
			overallPromo = PromoBlockStrict
			overallRisk = RiskHigh
		} else if lSym.EvidenceLevel == lifecycle.EvidenceUnknown || lSym.EvidenceLevel == "" || len(lSym.Sources) == 0 {
			symEntry.PointInTimeStatus = SymStatusMissingLifecycle
			symEntry.PromotionBlockingReasons = append(symEntry.PromotionBlockingReasons, "EVIDENCE_MISSING")
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolMissingLifecycle,
				Severity:        "ERROR",
				Reason:          "Lifecycle evidence is entirely missing",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    lSym.Symbol,
				BlocksPromotion: true,
				RecommendedFix:  "Provide lifecycle evidence",
			})
			overallStatus = StatusPitNotEligible
			overallPromo = PromoBlockStrict
			overallRisk = RiskHigh
		} else if lSym.EvidenceLevel == lifecycle.EvidenceLocalDataFirstSeen {
			symEntry.PointInTimeStatus = SymStatusLocalDataOnly
			symEntry.PromotionBlockingReasons = append(symEntry.PromotionBlockingReasons, "LOCAL_DATA_ONLY")
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolLocalDataOnly,
				Severity:        "ERROR",
				Reason:          "Evidence relies solely on local data presence, not actual exchange listing data",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    lSym.Symbol,
				BlocksPromotion: true,
				RecommendedFix:  "Provide exchange metadata snapshot evidence",
			})
			overallStatus = StatusPitNotEligible
			overallPromo = PromoBlockStrict
			overallRisk = RiskHigh
		} else if lSym.EvidenceLevel == lifecycle.EvidenceCurrentActiveOnly {
			symEntry.PointInTimeStatus = SymStatusCurrentOnly
			symEntry.PromotionBlockingReasons = append(symEntry.PromotionBlockingReasons, "CURRENT_ONLY")
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolCurrentOnly,
				Severity:        "ERROR",
				Reason:          "Evidence relies solely on current active exchange state",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    lSym.Symbol,
				BlocksPromotion: true,
				RecommendedFix:  "Provide historical exchange metadata snapshot evidence",
			})
			overallStatus = StatusPitNotEligible
			overallPromo = PromoBlockStrict
			overallRisk = RiskHigh
		} else if symEntry.UnverifiedSnapshotCount > 0 {
			symEntry.PointInTimeStatus = SymStatusUnverifiedOnly
			symEntry.PromotionBlockingReasons = append(symEntry.PromotionBlockingReasons, "UNVERIFIED_EVIDENCE")
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolUnverifiedOnly,
				Severity:        "ERROR",
				Reason:          "Evidence relies on unverified user backfill",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    lSym.Symbol,
				BlocksPromotion: true,
				RecommendedFix:  "Provide source-observed official exchange archive evidence; compatibility flags do not authorize strict promotion",
			})
			overallStatus = StatusPitNotEligible
			overallPromo = PromoBlockStrict
			overallRisk = RiskHigh
		} else if symEntry.MissingObservedTimeCount > 0 {
			symEntry.PointInTimeStatus = SymStatusPartialForWindow
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolObservedTimeMissing,
				Severity:        "ERROR",
				Reason:          "Some snapshot evidence is missing observed time",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    lSym.Symbol,
				BlocksPromotion: true,
				RecommendedFix:  "Provide snapshot evidence with known observed times",
			})
			overallPromo = PromoBlockStrict
			overallStatus = StatusPitNotEligible
			overallRisk = RiskHigh
		} else {
			symEntry.PointInTimeStatus = SymStatusVerifiedForWindow
		}

		if (lSym.DelistedAtUTC == "" || strings.EqualFold(lSym.DelistedAtUTC, StatusUnknown)) && strings.Contains(um.UniversePolicy, "POINT_IN_TIME") {
			// Without delisting evidence, we keep survivorship risk elevated
			overallRisk = RiskHigh
		}

		report.Symbols = append(report.Symbols, symEntry)
	}

	report.OverallStatus = overallStatus
	report.PromotionRecommendation = overallPromo
	report.SurvivorshipBiasRisk = overallRisk

	if overallStatus == StatusPitNotEligible {
		report.Validation.IsValid = false
	}

	// Compute hashes
	h, err := report.ComputeCoverageHash()
	if err != nil {
		return err
	}
	report.Hashes.CoverageHash = h

	rh, err := report.ComputeReportHash()
	if err != nil {
		return err
	}
	report.Hashes.ReportHash = rh

	return nil
}

func conservativePITRisk(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case RiskLow:
		return RiskLow
	case RiskMedium:
		return RiskMedium
	default:
		return RiskHigh
	}
}

func isTrustedPITSource(confidence string) bool {
	switch strings.ToUpper(strings.TrimSpace(confidence)) {
	case exchange_meta.TrustLevelOfficialArchive, exchange_meta.TrustLevelExchangeRawResponseArchive:
		return true
	default:
		return false
	}
}

func isKnownPITTimestamp(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, StatusUnknown) {
		return false
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func findLifecycleSymbol(lm *lifecycle.Manifest, sym string) (lifecycle.SymbolEntry, bool) {
	for _, s := range lm.Symbols {
		if s.Symbol == sym {
			return s, true
		}
	}
	return lifecycle.SymbolEntry{}, false
}

func copyStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	c := s
	return &c
}
