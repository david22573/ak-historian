package universe

import (
	"regexp"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/lifecycle"
)

// Validate validates the universe manifest and populates warnings.
func Validate(m *Manifest) {
	m.Validation.IsValid = true
	warnings := append([]Warning{}, m.Warnings...)
	symbolRE := regexp.MustCompile(`^[A-Z0-9]{3,30}$`)

	// Empty universe
	if len(m.Symbols) == 0 {
		warnings = append(warnings, Warning{
			Code:    CodeUniverseEmpty,
			Message: "Universe contains no symbols",
		})
		m.Validation.IsValid = false
	}

	seen := make(map[string]bool)
	for _, sym := range m.Symbols {
		symbol := strings.TrimSpace(sym.Symbol)
		if seen[symbol] {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseDuplicateSymbol,
				Target:  symbol,
				Message: "Duplicate symbol found",
			})
			m.Validation.IsValid = false
		}
		seen[symbol] = true

		if !symbolRE.MatchString(symbol) {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseSymbolMalformed,
				Target:  symbol,
				Message: "Malformed symbol",
			})
			m.Validation.IsValid = false
		}
		if m.LifecycleHash != "" && sym.Source != "asset_lifecycle_manifest" {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseSymbolMissingLifecycle,
				Target:  symbol,
				Message: "Universe symbol is missing from asset lifecycle manifest",
			})
			m.Validation.IsValid = false
		}
		if m.LifecycleHash != "" && !sym.ActiveDuringWindow {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseSymbolNotActiveDuringWindow,
				Target:  symbol,
				Message: "Lifecycle evidence shows symbol is not active during universe window",
			})
			m.Validation.IsValid = false
		}
	}

	start, startOK := parseOptionalTime(m.EffectiveStartUTC)
	end, endOK := parseOptionalTime(m.EffectiveEndUTC)
	if !startOK || !endOK || (!start.IsZero() && !end.IsZero() && start.After(end)) {
		warnings = append(warnings, Warning{
			Code:    CodeUniverseEffectiveWindowInvalid,
			Message: "Effective time window is invalid",
		})
		m.Validation.IsValid = false
	}

	if m.LifecycleHash == "" && isPointInTimePolicy(m.UniversePolicy) {
		warnings = append(warnings, Warning{
			Code:    CodeUniverseLifecycleManifestMissing,
			Message: "Point-in-time universe policy requires asset lifecycle evidence",
		})
		m.Validation.IsValid = false
	}

	if m.LifecycleHash != "" {
		if m.ExchangeMetadataSnapshotCurrentOnly {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseSnapshotArchiveCurrentOnly,
				Message: "Exchange metadata snapshot evidence is current-only and does not prove historical universe membership",
			})
		}
		if m.ExchangeMetadataSnapshotHash != "" || m.ExchangeMetadataSnapshotManifestHash != "" {
			switch m.PointInTimeCoverageStatus {
			case "COVERS_WINDOW":
				// Covered snapshot windows can support only the covered interval; listing/delisting proof checks still apply below.
			case "CURRENT_ONLY":
				warnings = append(warnings, Warning{
					Code:    CodeUniversePointInTimeEvidencePartial,
					Message: "Exchange metadata snapshot evidence is current-only, not full point-in-time archive coverage",
				})
			default:
				warnings = append(warnings, Warning{
					Code:    CodeUniverseSnapshotArchiveDoesNotCoverWindow,
					Message: "Exchange metadata snapshot archive does not cover the full universe effective window",
				})
				warnings = append(warnings, Warning{
					Code:    CodeUniversePointInTimeEvidencePartial,
					Message: "Point-in-time evidence from exchange metadata snapshots is partial",
				})
			}
		}
		if m.ListingEvidenceStatus != lifecycle.ListingEvidenceVerified {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseListingEvidenceMissing,
				Message: "Lifecycle manifest does not provide verified listing evidence for all symbols",
			})
			if m.ExchangeMetadataSnapshotHash != "" || m.ExchangeMetadataSnapshotManifestHash != "" {
				warnings = append(warnings, Warning{
					Code:    CodeUniverseListingNotProvenBySnapshots,
					Message: "Exchange metadata snapshots do not prove listing dates for all symbols",
				})
			}
		}
		if m.DelistingEvidenceStatus != lifecycle.DelistingEvidenceVerified {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseDelistingEvidenceMissing,
				Message: "Lifecycle manifest does not provide verified delisting evidence for all symbols",
			})
			if m.ExchangeMetadataSnapshotHash != "" || m.ExchangeMetadataSnapshotManifestHash != "" {
				warnings = append(warnings, Warning{
					Code:    CodeUniverseDelistingNotProvenBySnapshots,
					Message: "Exchange metadata snapshots do not prove delisting dates for all symbols",
				})
			}
		}
		if m.SurvivorshipSupportStatus != lifecycle.SupportLowSupported {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseLifecycleEvidenceWeak,
				Message: "Lifecycle evidence is insufficient to lower survivorship risk",
			})
		}
		for _, sym := range m.Symbols {
			if sym.ListedAtUTC == nil {
				warnings = append(warnings, Warning{
					Code:    CodeUniverseListingEvidenceMissing,
					Target:  sym.Symbol,
					Message: "Symbol listing date is unknown",
				})
			} else if !end.IsZero() {
				listed, ok := parseOptionalTime(*sym.ListedAtUTC)
				if ok && !listed.IsZero() && listed.After(end) {
					warnings = append(warnings, Warning{
						Code:    CodeUniverseSymbolNotActiveDuringWindow,
						Target:  sym.Symbol,
						Message: "Symbol listed after universe window",
					})
					m.Validation.IsValid = false
				}
			}
			if sym.DelistedAtUTC == nil {
				warnings = append(warnings, Warning{
					Code:    CodeUniverseDelistingEvidenceMissing,
					Target:  sym.Symbol,
					Message: "Symbol delisting date is unknown",
				})
			} else if !start.IsZero() {
				delisted, ok := parseOptionalTime(*sym.DelistedAtUTC)
				if ok && !delisted.IsZero() && delisted.Before(start) {
					warnings = append(warnings, Warning{
						Code:    CodeUniverseSymbolNotActiveDuringWindow,
						Target:  sym.Symbol,
						Message: "Symbol delisted before universe window",
					})
					m.Validation.IsValid = false
				}
			}
		}
	}

	// Policy checks
	validPolicies := map[string]bool{
		PolicyPointInTimeExchangeUniverse:  true,
		PolicyPointInTimeVolumeFiltered:    true,
		PolicyPointInTimeMarketCapFiltered: true,
		PolicyExplicitSymbolList:           true,
		PolicyCurrentActiveSymbolList:      true,
		PolicyLocalDataDiscoveredSymbols:   true,
		PolicyUnknown:                      true,
	}

	if !validPolicies[m.UniversePolicy] || m.UniversePolicy == PolicyUnknown {
		warnings = append(warnings, Warning{
			Code:    CodeUniversePolicyUnknown,
			Message: "Unknown universe policy: " + m.UniversePolicy,
		})
		m.Validation.IsValid = false
	}

	if m.UniversePolicy == PolicyExplicitSymbolList {
		warnings = append(warnings, Warning{
			Code:    CodeUniverseExplicitSymbolListSurvivorshipRisk,
			Message: "Explicit symbol lists carry high survivorship bias risk",
		})
		if m.SurvivorshipBiasRisk == RiskLow {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseLowRiskUnproven,
				Message: "Explicit symbol list cannot have LOW survivorship risk",
			})
			m.SurvivorshipBiasRisk = RiskHigh
		}
	}

	if m.UniversePolicy == PolicyCurrentActiveSymbolList {
		warnings = append(warnings, Warning{
			Code:    CodeUniverseCurrentActiveSurvivorshipRisk,
			Message: "Current active symbol lists are inherently survivorship biased",
		})
		if m.SurvivorshipBiasRisk == RiskLow {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseLowRiskUnproven,
				Message: "Current active symbol list cannot have LOW survivorship risk",
			})
			m.SurvivorshipBiasRisk = RiskHigh
		}
	}

	if m.UniversePolicy == PolicyLocalDataDiscoveredSymbols {
		warnings = append(warnings, Warning{
			Code:    CodeUniverseLocalDataDiscoveryNotPointInTime,
			Message: "Local data discovery is not guaranteed to be point-in-time",
		})
		if m.SurvivorshipBiasRisk == RiskLow {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseLowRiskUnproven,
				Message: "Local data discovery cannot have LOW survivorship risk without proof",
			})
			m.SurvivorshipBiasRisk = RiskMedium
		}
	}

	if m.IncludesDelistedAssets == "true" && !hasPointInTimeListingEvidence(m) {
		warnings = append(warnings, Warning{
			Code:    CodeUniverseDelistedStatusUnknown,
			Message: "Delisted asset inclusion is claimed without listing/delisting source evidence",
		})
		if m.SurvivorshipBiasRisk == RiskLow {
			warnings = append(warnings, Warning{
				Code:    CodeUniverseLowRiskUnproven,
				Message: "Cannot claim LOW risk without point-in-time listing/delisting evidence",
			})
			m.SurvivorshipBiasRisk = RiskMedium
		}
		m.Validation.IsValid = false
	} else if m.IncludesDelistedAssets == "unknown" {
		warnings = append(warnings, Warning{
			Code:    CodeUniverseDelistedStatusUnknown,
			Message: "Delisted asset inclusion is unknown",
		})
	}

	if m.SurvivorshipBiasRisk == RiskLow && !hasPointInTimeListingEvidence(m) {
		warnings = append(warnings, Warning{
			Code:    CodeUniverseLowRiskUnproven,
			Message: "Cannot claim LOW risk without point-in-time listing/delisting evidence",
		})
		m.SurvivorshipBiasRisk = defaultRiskForPolicy(m.UniversePolicy)
		if m.SurvivorshipBiasRisk == RiskLow {
			m.SurvivorshipBiasRisk = RiskUnknown
		}
	}

	m.Warnings = warnings
}

func parseOptionalTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, true
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func hasPointInTimeListingEvidence(m *Manifest) bool {
	switch m.UniversePolicy {
	case PolicyPointInTimeExchangeUniverse, PolicyPointInTimeVolumeFiltered, PolicyPointInTimeMarketCapFiltered:
	default:
		return false
	}
	if m.IncludesDelistedAssets != "true" {
		return false
	}
	if m.LifecycleHash != "" &&
		m.ListingEvidenceStatus == lifecycle.ListingEvidenceVerified &&
		m.DelistingEvidenceStatus == lifecycle.DelistingEvidenceVerified &&
		m.SurvivorshipSupportStatus == lifecycle.SupportLowSupported {
		return true
	}
	source := strings.ToLower(m.SourceType)
	if !strings.Contains(source, "point") && !strings.Contains(source, "listing") && !strings.Contains(source, "delisting") {
		return false
	}
	for _, sym := range m.Symbols {
		if sym.ListedAtUTC == nil {
			return false
		}
	}
	return len(m.Symbols) > 0
}

func isPointInTimePolicy(policy string) bool {
	switch policy {
	case PolicyPointInTimeExchangeUniverse, PolicyPointInTimeVolumeFiltered, PolicyPointInTimeMarketCapFiltered:
		return true
	default:
		return false
	}
}
