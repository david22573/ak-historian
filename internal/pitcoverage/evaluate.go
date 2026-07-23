package pitcoverage

import (
	"strings"

	"github.com/david22573/ak-historian/internal/exchange_meta"
	"github.com/david22573/ak-historian/internal/lifecycle"
	"github.com/david22573/ak-historian/internal/universe"
)

func (b *Builder) evaluateCoverage(report *Report, lm *lifecycle.Manifest, um *universe.Manifest, sm *exchange_meta.SnapshotManifest) error {
	overallStatus := StatusPitEligible
	overallPromo := PromoAllowStrict
	overallRisk := RiskLow

	if um.SurvivorshipBiasRisk == RiskHigh {
		overallRisk = RiskHigh
	} else if um.SurvivorshipBiasRisk == RiskMedium {
		overallRisk = RiskMedium
	}

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
			if src.Confidence == exchange_meta.TrustLevelUserProvidedUnverified {
				symEntry.UnverifiedSnapshotCount++
			}
			if src.ObservedAtUTC != "" {
				symEntry.ObservedSnapshotCount++
			} else {
				symEntry.MissingObservedTimeCount++
			}
			symEntry.SnapshotPresenceCount++
		}

		// Evaluate point-in-time status for the symbol
		if lSym.EvidenceLevel == lifecycle.EvidenceUnknown || lSym.EvidenceLevel == "" {
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
		} else if symEntry.UnverifiedSnapshotCount > 0 && !b.AllowUnverified {
			symEntry.PointInTimeStatus = SymStatusUnverifiedOnly
			symEntry.PromotionBlockingReasons = append(symEntry.PromotionBlockingReasons, "UNVERIFIED_EVIDENCE")
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolUnverifiedOnly,
				Severity:        "ERROR",
				Reason:          "Evidence relies on unverified user backfill",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    lSym.Symbol,
				BlocksPromotion: true,
				RecommendedFix:  "Provide verified exchange metadata snapshot evidence or set --allow-unverified",
			})
			overallStatus = StatusPitNotEligible
			overallPromo = PromoBlockStrict
		} else if symEntry.UnverifiedSnapshotCount > 0 && b.AllowUnverified {
			symEntry.PointInTimeStatus = SymStatusUnverifiedOnly
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolUnverifiedOnly,
				Severity:        "WARNING",
				Reason:          "Evidence relies on unverified user backfill, strict promotion claims disabled",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    lSym.Symbol,
				BlocksPromotion: false,
				RecommendedFix:  "Provide verified exchange metadata snapshot evidence",
			})
			if overallRisk == RiskLow {
				overallRisk = RiskMedium // do not lower risk to LOW if unverified
			}
			if overallPromo == PromoAllowStrict {
				overallPromo = PromoDowngrade
			}
			if overallStatus == StatusPitEligible {
				overallStatus = StatusPitPartial
			}
		} else if symEntry.MissingObservedTimeCount > 0 {
			symEntry.PointInTimeStatus = SymStatusPartialForWindow
			symEntry.Warnings = append(symEntry.Warnings, Warning{
				Code:            CodeSymbolObservedTimeMissing,
				Severity:        "WARNING",
				Reason:          "Some snapshot evidence is missing observed time",
				TargetArtifact:  "lifecycle_manifest",
				TargetSymbol:    lSym.Symbol,
				BlocksPromotion: false, // downgrades it
				RecommendedFix:  "Provide snapshot evidence with known observed times",
			})
			if overallPromo == PromoAllowStrict {
				overallPromo = PromoDowngrade
			}
			if overallStatus == StatusPitEligible {
				overallStatus = StatusPitPartial
			}
		} else {
			symEntry.PointInTimeStatus = SymStatusVerifiedForWindow
		}

		if lSym.DelistedAtUTC == "" && strings.Contains(um.UniversePolicy, "POINT_IN_TIME") {
			// Without delisting evidence, we keep survivorship risk elevated
			if overallRisk == RiskLow {
				overallRisk = RiskMedium
			}
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
