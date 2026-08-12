package pitcoverage

import (
	"fmt"
	"strings"
)

// ValidateReport verifies the complete legacy report before it is embedded in
// another artifact. Legacy PIT reports are diagnostic-only and can never grant
// strict promotion; canonical research identity is the sole strict authority.
func ValidateReport(report *Report) error {
	if report == nil || report.SchemaVersion != "1.0.0" || report.ReportVersion != "1.0.0" || report.CoverageReportID == "" {
		return fmt.Errorf("invalid PIT coverage report contract")
	}
	claimedCoverage := report.Hashes.CoverageHash
	claimedReport := report.Hashes.ReportHash
	wantCoverage, err := report.ComputeCoverageHash()
	if err != nil {
		return fmt.Errorf("compute PIT coverage hash: %w", err)
	}
	if claimedCoverage == "" || claimedCoverage != wantCoverage {
		return fmt.Errorf("PIT coverage hash mismatch")
	}
	wantReport, err := report.ComputeReportHash()
	if err != nil {
		return fmt.Errorf("compute PIT report hash: %w", err)
	}
	if claimedReport == "" || claimedReport != wantReport {
		return fmt.Errorf("PIT report hash mismatch")
	}
	if report.PromotionRecommendation == PromoAllowStrict {
		return fmt.Errorf("legacy PIT report cannot authorize strict promotion")
	}
	if report.SurvivorshipBiasRisk != RiskLow && report.SurvivorshipBiasRisk != RiskMedium && report.SurvivorshipBiasRisk != RiskHigh {
		return fmt.Errorf("invalid PIT survivorship risk %q", report.SurvivorshipBiasRisk)
	}
	switch report.OverallStatus {
	case StatusPitEligible:
		if !report.Validation.IsValid || report.PromotionRecommendation != PromoExploratoryOnly {
			return fmt.Errorf("eligible legacy PIT report has inconsistent validation or promotion status")
		}
		for _, symbol := range report.Symbols {
			if symbol.PointInTimeStatus != SymStatusVerifiedForWindow || symbol.EvidenceLevel == "" || strings.EqualFold(symbol.EvidenceLevel, StatusUnknown) || len(symbol.PromotionBlockingReasons) > 0 {
				return fmt.Errorf("eligible legacy PIT report contains ineligible symbol %s", symbol.Symbol)
			}
		}
	case StatusPitPartial, StatusPitNotEligible:
		if report.PromotionRecommendation != PromoBlockStrict && report.PromotionRecommendation != PromoDowngrade && report.PromotionRecommendation != PromoExploratoryOnly {
			return fmt.Errorf("ineligible legacy PIT report has invalid promotion status")
		}
	case StatusUnknown:
		return fmt.Errorf("UNKNOWN PIT coverage is ineligible")
	default:
		return fmt.Errorf("unrecognized PIT coverage status %q", report.OverallStatus)
	}
	return nil
}
