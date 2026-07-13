package pitarchive

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type canonicalCoverageReport struct {
	PolicyID                   string       `json:"policy_id"`
	PolicySchemaVersion        string       `json:"policy_schema_version"`
	ExpectedPartitions         []string     `json:"expected_partitions"`
	ObservedPartitions         []string     `json:"observed_partitions"`
	MissingPartitions          []string     `json:"missing_partitions"`
	DuplicatePartitions        []string     `json:"duplicate_partitions"`
	OutOfWindowPartitions      []string     `json:"out_of_window_partitions"`
	CoverageRatio              string       `json:"coverage_ratio"`
	MaximumObservedGapSeconds  int64        `json:"maximum_observed_gap_seconds"`
	MaximumPermittedGapSeconds int64        `json:"maximum_permitted_gap_seconds"`
	StrictVerdict              CheckVerdict `json:"strict_verdict"`
}

func sortedStrings(values []string) []string {
	result := make([]string, len(values))
	copy(result, values)
	sort.Strings(result)
	return result
}

func canonicalCoverage(report CoverageReport) canonicalCoverageReport {
	return canonicalCoverageReport{
		PolicyID: report.PolicyID, PolicySchemaVersion: report.PolicySchemaVersion,
		ExpectedPartitions: sortedStrings(report.ExpectedPartitions), ObservedPartitions: sortedStrings(report.ObservedPartitions),
		MissingPartitions: sortedStrings(report.MissingPartitions), DuplicatePartitions: sortedStrings(report.DuplicatePartitions),
		OutOfWindowPartitions: sortedStrings(report.OutOfWindowPartitions), CoverageRatio: report.CoverageRatio,
		MaximumObservedGapSeconds: report.MaximumObservedGapSeconds, MaximumPermittedGapSeconds: report.MaximumPermittedGapSeconds,
		StrictVerdict: report.StrictVerdict,
	}
}

func uniqueStrings(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) || !uniqueStrings(left) || !uniqueStrings(right) {
		return false
	}
	want := make(map[string]struct{}, len(left))
	for _, value := range left {
		want[value] = struct{}{}
	}
	for _, value := range right {
		if _, exists := want[value]; !exists {
			return false
		}
	}
	return true
}

func ComputeEvidenceIntegrityHash(evidence EvidenceEnvelope) (string, error) {
	canonical := struct {
		SchemaVersion             string                  `json:"schema_version"`
		EvidenceID                string                  `json:"evidence_id"`
		DatasetID                 string                  `json:"dataset_id"`
		DatasetVersion            string                  `json:"dataset_version"`
		ResearchWindowStart       string                  `json:"research_window_start"`
		ResearchWindowEnd         string                  `json:"research_window_end"`
		EvaluationCutoff          string                  `json:"evaluation_cutoff"`
		ManifestID                string                  `json:"manifest_id"`
		ManifestHash              string                  `json:"manifest_hash"`
		ArchiveID                 string                  `json:"archive_id"`
		CoveragePolicyVersion     string                  `json:"coverage_policy_version"`
		ManifestValidationVerdict CheckVerdict            `json:"manifest_validation_verdict"`
		SnapshotIntegrity         SnapshotIntegrityReport `json:"snapshot_integrity"`
		Coverage                  canonicalCoverageReport `json:"coverage"`
		SnapshotCount             int                     `json:"snapshot_count"`
		SnapshotSetDigest         string                  `json:"snapshot_set_digest"`
		Availability              struct {
			PolicyID         string       `json:"policy_id"`
			EvaluationCutoff string       `json:"evaluation_cutoff"`
			AcceptedCount    int          `json:"accepted_count"`
			RejectedCount    int          `json:"rejected_count"`
			StrictVerdict    CheckVerdict `json:"strict_verdict"`
		} `json:"availability"`
		FinalVerdict           Verdict `json:"final_verdict"`
		StrictPromotionAllowed bool    `json:"strict_promotion_allowed"`
		GeneratedAt            string  `json:"generated_at"`
		HistorianBuild         string  `json:"historian_build"`
	}{
		SchemaVersion: evidence.SchemaVersion, EvidenceID: evidence.EvidenceID, DatasetID: evidence.DatasetID,
		DatasetVersion: evidence.DatasetVersion, ResearchWindowStart: canonicalTime(evidence.ResearchWindowStart),
		ResearchWindowEnd: canonicalTime(evidence.ResearchWindowEnd), EvaluationCutoff: canonicalTime(evidence.EvaluationCutoff),
		ManifestID: evidence.ManifestID, ManifestHash: strings.ToLower(evidence.ManifestHash), ArchiveID: evidence.ArchiveID,
		CoveragePolicyVersion: evidence.CoveragePolicyVersion, ManifestValidationVerdict: evidence.ManifestValidationVerdict,
		SnapshotIntegrity: evidence.SnapshotIntegrity, Coverage: canonicalCoverage(evidence.Coverage), SnapshotCount: evidence.SnapshotCount,
		SnapshotSetDigest: strings.ToLower(evidence.SnapshotSetDigest), FinalVerdict: evidence.FinalVerdict,
		StrictPromotionAllowed: evidence.StrictPromotionAllowed, GeneratedAt: canonicalTime(evidence.GeneratedAt), HistorianBuild: evidence.HistorianBuild,
	}
	canonical.Availability.PolicyID = evidence.Availability.PolicyID
	canonical.Availability.EvaluationCutoff = canonicalTime(evidence.Availability.EvaluationCutoff)
	canonical.Availability.AcceptedCount = evidence.Availability.AcceptedCount
	canonical.Availability.RejectedCount = evidence.Availability.RejectedCount
	canonical.Availability.StrictVerdict = evidence.Availability.StrictVerdict
	return digestCanonical(canonical)
}

func VerifyEvidence(evidence EvidenceEnvelope) []Failure {
	var failures []Failure
	if evidence.SchemaVersion != EvidenceSchemaVersion {
		failures = append(failures, Failure{Code: ReasonEvidenceSchemaUnsupported, Message: "unsupported PIT evidence schema"})
	}
	if strings.TrimSpace(evidence.EvidenceID) == "" || strings.TrimSpace(evidence.DatasetID) == "" || strings.TrimSpace(evidence.DatasetVersion) == "" ||
		strings.TrimSpace(evidence.ManifestID) == "" || strings.TrimSpace(evidence.ArchiveID) == "" || strings.TrimSpace(evidence.CoveragePolicyVersion) == "" ||
		strings.TrimSpace(evidence.HistorianBuild) == "" || evidence.ResearchWindowStart.IsZero() || evidence.ResearchWindowEnd.IsZero() ||
		evidence.EvaluationCutoff.IsZero() || evidence.GeneratedAt.IsZero() || evidence.SnapshotCount <= 0 || !validSHA256(evidence.ManifestHash) || !validSHA256(evidence.SnapshotSetDigest) {
		failures = append(failures, Failure{Code: ReasonEvidenceSecurityFieldMissing, Message: "PIT evidence is missing a security-relevant field"})
	}
	switch evidence.FinalVerdict {
	case VerdictEligible, VerdictIneligible, VerdictEvidenceIncomplete, VerdictEvidenceCorrupt, VerdictDiagnosticOnly:
	default:
		failures = append(failures, Failure{Code: ReasonEvidenceSecurityFieldMissing, Message: "PIT evidence contains an unknown final verdict"})
	}
	expected, err := ComputeEvidenceIntegrityHash(evidence)
	if err != nil || !compareDigest(evidence.IntegrityHash, expected) {
		failures = append(failures, Failure{Code: ReasonEvidenceIntegrityHashMismatch, Message: "PIT evidence integrity hash does not match canonical contents"})
	}
	allChecksPass := evidence.ManifestValidationVerdict == CheckPass &&
		evidence.SnapshotIntegrity.StrictVerdict == CheckPass && evidence.SnapshotIntegrity.VerifiedCount == evidence.SnapshotCount && evidence.SnapshotIntegrity.RejectedCount == 0 &&
		evidence.Coverage.StrictVerdict == CheckPass && evidence.Coverage.PolicySchemaVersion == evidence.CoveragePolicyVersion && strings.TrimSpace(evidence.Coverage.PolicyID) != "" &&
		len(evidence.Coverage.ExpectedPartitions) == evidence.SnapshotCount && len(evidence.Coverage.ObservedPartitions) == evidence.SnapshotCount &&
		sameStringSet(evidence.Coverage.ExpectedPartitions, evidence.Coverage.ObservedPartitions) &&
		len(evidence.Coverage.MissingPartitions) == 0 && len(evidence.Coverage.DuplicatePartitions) == 0 && len(evidence.Coverage.OutOfWindowPartitions) == 0 && evidence.Coverage.CoverageRatio == "1.000000" &&
		evidence.Availability.StrictVerdict == CheckPass && strings.TrimSpace(evidence.Availability.PolicyID) != "" &&
		evidence.Availability.EvaluationCutoff.Equal(evidence.EvaluationCutoff) && evidence.Availability.AcceptedCount == evidence.SnapshotCount && evidence.Availability.RejectedCount == 0
	expectedEvidenceID, evidenceIDError := evidenceID(evidence.ManifestHash, evidence.EvaluationCutoff, evidence.SnapshotSetDigest)
	if evidenceIDError != nil || evidence.EvidenceID != expectedEvidenceID {
		failures = append(failures, Failure{Code: ReasonEvidenceSecurityFieldMissing, Message: "PIT evidence ID is inconsistent with its bound identities"})
	}
	if !evidence.ResearchWindowEnd.After(evidence.ResearchWindowStart) {
		failures = append(failures, Failure{Code: ReasonEvidenceSecurityFieldMissing, Message: "PIT evidence research-window bounds are invalid"})
	}
	if evidence.GeneratedAt.Before(evidence.EvaluationCutoff) {
		failures = append(failures, Failure{Code: ReasonEvidenceSecurityFieldMissing, Message: "PIT evidence cannot be generated before its evaluation cutoff"})
	}
	if evidence.FinalVerdict != VerdictEligible && evidence.FinalVerdict != VerdictDiagnosticOnly && allChecksPass {
		failures = append(failures, Failure{Code: ReasonEvidenceStrictPromotionInvalid, Message: "noneligible PIT evidence must identify at least one failed bound check"})
	}
	if evidence.StrictPromotionAllowed != (evidence.FinalVerdict == VerdictEligible && allChecksPass) {
		failures = append(failures, Failure{Code: ReasonEvidenceStrictPromotionInvalid, Message: "strict promotion flag is inconsistent with PIT verdict and checks"})
	}
	if evidence.FinalVerdict == VerdictEligible && !evidence.StrictPromotionAllowed {
		failures = append(failures, Failure{Code: ReasonEvidenceStrictPromotionInvalid, Message: "PIT_ELIGIBLE requires strict promotion approval"})
	}
	return failures
}

func WriteEvidence(path string, evidence EvidenceEnvelope) error {
	if failures := VerifyEvidence(evidence); len(failures) > 0 {
		return fmt.Errorf("refuse to write invalid PIT evidence: %s", failures[0].Message)
	}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal PIT evidence: %w", err)
	}
	return writeAtomic(path, append(data, '\n'), DefaultMaxEvidenceBytes)
}

func WriteEvaluationResult(path string, result EvaluationResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal PIT evaluation result: %w", err)
	}
	return writeAtomic(path, append(data, '\n'), DefaultMaxEvidenceBytes)
}
