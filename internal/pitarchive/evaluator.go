package pitarchive

import (
	"fmt"
	"strings"
	"time"
)

func Evaluate(options EvaluateOptions) (EvaluationResult, error) {
	result := EvaluationResult{SchemaVersion: EvaluationResultSchemaVersion, Failures: []Failure{}}
	if options.MaxManifestBytes <= 0 {
		options.MaxManifestBytes = DefaultMaxManifestBytes
	}
	if options.MaxSnapshotBytes <= 0 {
		options.MaxSnapshotBytes = DefaultMaxSnapshotBytes
	}
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	optionFailures := validateOptions(options)
	if len(optionFailures) > 0 {
		result.Failures = append(result.Failures, optionFailures...)
		result.Verdict = requestedVerdict(options.Strict, result.Failures)
		return result, nil
	}

	manifest, err := LoadManifest(options.ManifestPath, options.MaxManifestBytes)
	if err != nil {
		result.Failures = append(result.Failures, failureFromError(err))
		result.Verdict = requestedVerdict(options.Strict, result.Failures)
		return result, nil
	}
	manifestFailures := validateManifestStructure(manifest)
	manifestFailures = append(manifestFailures, validateManifestExpectations(manifest, options)...)
	if len(manifestFailures) > 0 {
		result.Failures = append(result.Failures, manifestFailures...)
		result.Verdict = requestedVerdict(options.Strict, result.Failures)
		return result, nil
	}

	integrity, accepted, snapshotFailures := verifySnapshots(options.ArchiveRoot, manifest.Snapshots, options.MaxSnapshotBytes)
	coverage, coverageFailures := verifyCoverage(manifest)
	availability, availabilityFailures := verifyAvailability(manifest, options.EvaluationCutoff, options.Now)
	result.Failures = append(result.Failures, snapshotFailures...)
	result.Failures = append(result.Failures, coverageFailures...)
	result.Failures = append(result.Failures, availabilityFailures...)
	result.Verdict = requestedVerdict(options.Strict, result.Failures)
	result.StrictPromotionAllowed = options.Strict && result.Verdict == VerdictEligible

	snapshotSetDigest, err := ComputeSnapshotSetDigest(accepted)
	if err != nil {
		return EvaluationResult{}, fmt.Errorf("compute accepted snapshot-set digest: %w", err)
	}
	evidenceID, err := evidenceID(manifest.ManifestHash, options.EvaluationCutoff, snapshotSetDigest)
	if err != nil {
		return EvaluationResult{}, err
	}
	evidence := EvidenceEnvelope{
		SchemaVersion: EvidenceSchemaVersion, EvidenceID: evidenceID, DatasetID: manifest.DatasetID, DatasetVersion: manifest.DatasetVersion,
		ResearchWindowStart: manifest.ResearchWindowStart, ResearchWindowEnd: manifest.ResearchWindowEnd, EvaluationCutoff: options.EvaluationCutoff,
		ManifestID: manifest.ManifestID, ManifestHash: manifest.ManifestHash, ArchiveID: manifest.ArchiveID,
		CoveragePolicyVersion: manifest.CoveragePolicy.SchemaVersion, ManifestValidationVerdict: CheckPass,
		SnapshotIntegrity: integrity, Coverage: coverage, SnapshotCount: len(accepted), SnapshotSetDigest: snapshotSetDigest,
		Availability: availability, FinalVerdict: result.Verdict, StrictPromotionAllowed: result.StrictPromotionAllowed,
		GeneratedAt: options.Now.UTC(), HistorianBuild: options.HistorianBuild,
	}
	evidence.IntegrityHash, err = ComputeEvidenceIntegrityHash(evidence)
	if err != nil {
		return EvaluationResult{}, fmt.Errorf("compute PIT evidence integrity hash: %w", err)
	}
	if failures := VerifyEvidence(evidence); len(failures) > 0 {
		result.Failures = append(result.Failures, failures...)
		result.Verdict = requestedVerdict(options.Strict, result.Failures)
		result.StrictPromotionAllowed = false
		evidence.FinalVerdict = result.Verdict
		evidence.StrictPromotionAllowed = false
		evidence.IntegrityHash, err = ComputeEvidenceIntegrityHash(evidence)
		if err != nil {
			return EvaluationResult{}, fmt.Errorf("recompute failed PIT evidence integrity hash: %w", err)
		}
	}
	result.Evidence = &evidence
	return result, nil
}

func validateOptions(options EvaluateOptions) []Failure {
	var failures []Failure
	if options.EvaluationCutoff.IsZero() {
		failures = append(failures, Failure{Code: ReasonEvaluationCutoffMissing, Message: "evaluation cutoff is required"})
	}
	if strings.TrimSpace(options.DatasetID) == "" || strings.TrimSpace(options.DatasetVersion) == "" || strings.TrimSpace(options.HistorianBuild) == "" {
		failures = append(failures, Failure{Code: ReasonManifestFieldMissing, Message: "evaluation dataset identity and historian build are required"})
	}
	if options.ResearchWindowStart.IsZero() || options.ResearchWindowEnd.IsZero() || !options.ResearchWindowEnd.After(options.ResearchWindowStart) {
		failures = append(failures, Failure{Code: ReasonManifestWindowMismatch, Message: "evaluation research-window bounds are required and must be ordered"})
	}
	if !options.EvaluationCutoff.IsZero() && options.EvaluationCutoff.After(options.Now) {
		failures = append(failures, Failure{Code: ReasonFutureSnapshotTimestamp, Message: "evaluation cutoff cannot be in the future"})
	}
	return failures
}

func requestedVerdict(strict bool, failures []Failure) Verdict {
	if !strict {
		return VerdictDiagnosticOnly
	}
	if len(failures) == 0 {
		return VerdictEligible
	}
	verdict := VerdictIneligible
	for _, failure := range failures {
		switch failure.Code {
		case ReasonManifestMissing, ReasonManifestEmpty, ReasonManifestUnreadable, ReasonManifestFieldMissing, ReasonSnapshotMissing,
			ReasonManifestSnapshotCountMismatch, ReasonEvaluationCutoffMissing, ReasonAvailabilityTimestampMissing:
			if verdict != VerdictEvidenceCorrupt {
				verdict = VerdictEvidenceIncomplete
			}
		case ReasonManifestMalformed, ReasonManifestSchemaUnsupported, ReasonManifestHashMismatch,
			ReasonSnapshotHashMismatch, ReasonSnapshotDuplicateID, ReasonSnapshotPartitionConflict,
			ReasonSnapshotSchemaUnsupported, ReasonSnapshotPathInvalid, ReasonSnapshotTooLarge, ReasonSnapshotSizeMismatch,
			ReasonCoveragePolicyUnsupported, ReasonCoveragePolicyInvalid, ReasonEvidenceSchemaUnsupported,
			ReasonEvidenceIntegrityHashMismatch, ReasonEvidenceStrictPromotionInvalid, ReasonEvidenceSecurityFieldMissing:
			verdict = VerdictEvidenceCorrupt
		}
	}
	return verdict
}
