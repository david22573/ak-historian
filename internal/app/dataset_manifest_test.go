package app

import (
	"testing"

	"github.com/david22573/ak-historian/internal/manifest"
	"github.com/david22573/ak-historian/internal/pitcoverage"
)

func TestDatasetManifestConsumesOnlyValidatedBlockedLegacyPIT(t *testing.T) {
	report := &pitcoverage.Report{
		SchemaVersion: "1.0.0", ReportVersion: "1.0.0", CoverageReportID: "pit-test",
		OverallStatus: pitcoverage.StatusPitNotEligible, PromotionRecommendation: pitcoverage.PromoBlockStrict,
		SurvivorshipBiasRisk: pitcoverage.RiskHigh,
		Symbols: []pitcoverage.SymbolEntry{{
			Symbol: "BTCUSDT", EvidenceLevel: "USER_PROVIDED_UNVERIFIED",
			PointInTimeStatus:        pitcoverage.SymStatusUnverifiedOnly,
			PromotionBlockingReasons: []string{"UNVERIFIED_EVIDENCE"},
		}},
		Windows:    []pitcoverage.Window{},
		Warnings:   []pitcoverage.Warning{},
		Validation: pitcoverage.Validation{IsValid: false},
	}
	report.Hashes.CoverageHash, _ = report.ComputeCoverageHash()
	report.Hashes.ReportHash, _ = report.ComputeReportHash()
	dataset := &manifest.DatasetManifest{Symbols: []string{}, Intervals: []string{}, Files: []manifest.FileEntry{}}
	if err := applyPITCoverageReport(dataset, report); err != nil {
		t.Fatal(err)
	}
	if dataset.Survivorship.PointInTimeCoverageStatus != pitcoverage.StatusPitNotEligible || dataset.Survivorship.PointInTimePromotionRecommendation != pitcoverage.PromoBlockStrict || dataset.Survivorship.SurvivorshipBiasRisk != pitcoverage.RiskHigh || dataset.Hashes.ManifestHash == "" {
		t.Fatalf("blocked PIT disposition was not preserved: %+v", dataset.Survivorship)
	}

	tampered := *report
	tampered.PromotionRecommendation = pitcoverage.PromoAllowStrict
	if err := applyPITCoverageReport(&manifest.DatasetManifest{}, &tampered); err == nil {
		t.Fatal("stale-hash strict-promotion tampering was accepted")
	}
	tampered.Hashes.CoverageHash, _ = tampered.ComputeCoverageHash()
	tampered.Hashes.ReportHash, _ = tampered.ComputeReportHash()
	if err := applyPITCoverageReport(&manifest.DatasetManifest{}, &tampered); err == nil {
		t.Fatal("rehashed legacy strict-promotion assertion was accepted")
	}
}
