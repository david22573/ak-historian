package pitarchive

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type coverageSegment struct {
	start time.Time
	end   time.Time
}

func validateCoveragePolicy(manifest SnapshotManifest) []Failure {
	policy := manifest.CoveragePolicy
	var failures []Failure
	if policy.SchemaVersion == "" {
		failures = append(failures, Failure{Code: ReasonManifestFieldMissing, Message: "coverage_policy.schema_version is required"})
	} else if policy.SchemaVersion != CoveragePolicySchemaVersion {
		failures = append(failures, Failure{Code: ReasonCoveragePolicyUnsupported, Message: "unsupported coverage policy schema"})
	}
	if strings.TrimSpace(policy.PolicyID) == "" || strings.TrimSpace(policy.PartitionModel) == "" {
		failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "coverage policy identity and partition model are required"})
	}
	if policy.MaximumGapSeconds == nil || (policy.MaximumGapSeconds != nil && (*policy.MaximumGapSeconds < 0 || *policy.MaximumGapSeconds > math.MaxInt64/int64(time.Second))) {
		failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "coverage policy maximum gap must be explicitly declared and nonnegative"})
	}
	if len(policy.Required) == 0 {
		failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "coverage policy requires at least one partition"})
	}

	segments := make([]coverageSegment, 0, len(policy.Required)+len(policy.Exceptions))
	keys := make(map[string]struct{}, len(policy.Required))
	for _, required := range policy.Required {
		if strings.TrimSpace(required.PartitionKey) == "" || required.EventTimeStart.IsZero() || required.EventTimeEnd.IsZero() || !required.EventTimeEnd.After(required.EventTimeStart) {
			failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "required partition fields are missing or unordered", PartitionKey: required.PartitionKey})
			continue
		}
		if _, exists := keys[required.PartitionKey]; exists {
			failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "coverage policy contains duplicate partition key", PartitionKey: required.PartitionKey})
		}
		keys[required.PartitionKey] = struct{}{}
		segments = append(segments, coverageSegment{required.EventTimeStart, required.EventTimeEnd})
	}
	for _, exception := range policy.Exceptions {
		if strings.TrimSpace(exception.ExceptionID) == "" || strings.TrimSpace(exception.Reason) == "" || exception.EventTimeStart.IsZero() || exception.EventTimeEnd.IsZero() || !exception.EventTimeEnd.After(exception.EventTimeStart) {
			failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "coverage exception must have identity, reason, and ordered bounds"})
			continue
		}
		segments = append(segments, coverageSegment{exception.EventTimeStart, exception.EventTimeEnd})
	}
	if len(segments) == 0 || policy.MaximumGapSeconds == nil || manifest.ResearchWindowStart.IsZero() || manifest.ResearchWindowEnd.IsZero() {
		return failures
	}
	sort.Slice(segments, func(i, j int) bool { return segments[i].start.Before(segments[j].start) })
	maximumGap := time.Duration(*policy.MaximumGapSeconds) * time.Second
	if segments[0].start.Before(manifest.ResearchWindowStart) || segments[len(segments)-1].end.After(manifest.ResearchWindowEnd) {
		failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "coverage policy contains out-of-window interval"})
	}
	if segments[0].start.Sub(manifest.ResearchWindowStart) > maximumGap || manifest.ResearchWindowEnd.Sub(segments[len(segments)-1].end) > maximumGap {
		failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "coverage policy does not span research-window boundaries"})
	}
	for i := 1; i < len(segments); i++ {
		if segments[i].start.Before(segments[i-1].end) {
			failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "coverage policy intervals overlap"})
			continue
		}
		if segments[i].start.Sub(segments[i-1].end) > maximumGap {
			failures = append(failures, Failure{Code: ReasonCoveragePolicyInvalid, Message: "coverage policy contains a gap larger than declared maximum"})
		}
	}
	return failures
}

func verifyCoverage(manifest SnapshotManifest) (CoverageReport, []Failure) {
	policy := manifest.CoveragePolicy
	report := CoverageReport{
		PolicyID: policy.PolicyID, PolicySchemaVersion: policy.SchemaVersion,
		ExpectedPartitions: []string{}, ObservedPartitions: []string{}, MissingPartitions: []string{},
		DuplicatePartitions: []string{}, OutOfWindowPartitions: []string{}, StrictVerdict: CheckPass,
	}
	if policy.MaximumGapSeconds != nil {
		report.MaximumPermittedGapSeconds = *policy.MaximumGapSeconds
	}
	expected := make(map[string]PartitionRequirement, len(policy.Required))
	for _, required := range policy.Required {
		expected[required.PartitionKey] = required
		report.ExpectedPartitions = append(report.ExpectedPartitions, required.PartitionKey)
	}
	observed := make(map[string]int, len(manifest.Snapshots))
	var failures []Failure
	for _, snapshot := range manifest.Snapshots {
		if snapshot.EventTimeStart.Before(manifest.ResearchWindowStart) || snapshot.EventTimeEnd.After(manifest.ResearchWindowEnd) {
			report.OutOfWindowPartitions = append(report.OutOfWindowPartitions, snapshot.PartitionKey)
			continue
		}
		required, exists := expected[snapshot.PartitionKey]
		if !exists || !required.EventTimeStart.Equal(snapshot.EventTimeStart) || !required.EventTimeEnd.Equal(snapshot.EventTimeEnd) {
			report.OutOfWindowPartitions = append(report.OutOfWindowPartitions, snapshot.PartitionKey)
			failures = append(failures, Failure{Code: ReasonSnapshotPartitionConflict, Message: "snapshot does not match its declared required partition", SnapshotID: snapshot.SnapshotID, PartitionKey: snapshot.PartitionKey})
			continue
		}
		observed[snapshot.PartitionKey]++
		if observed[snapshot.PartitionKey] == 1 {
			report.ObservedPartitions = append(report.ObservedPartitions, snapshot.PartitionKey)
		} else if observed[snapshot.PartitionKey] == 2 {
			report.DuplicatePartitions = append(report.DuplicatePartitions, snapshot.PartitionKey)
			failures = append(failures, Failure{Code: ReasonSnapshotPartitionConflict, Message: "multiple snapshots claim the same required partition", SnapshotID: snapshot.SnapshotID, PartitionKey: snapshot.PartitionKey})
		}
	}
	for key := range expected {
		if observed[key] == 0 {
			report.MissingPartitions = append(report.MissingPartitions, key)
		}
	}
	observedIntervals := make([]coverageSegment, 0, len(manifest.Snapshots))
	for _, snapshot := range manifest.Snapshots {
		required, exists := expected[snapshot.PartitionKey]
		if exists && required.EventTimeStart.Equal(snapshot.EventTimeStart) && required.EventTimeEnd.Equal(snapshot.EventTimeEnd) && observed[snapshot.PartitionKey] == 1 {
			observedIntervals = append(observedIntervals, coverageSegment{start: snapshot.EventTimeStart, end: snapshot.EventTimeEnd})
		}
	}
	sort.Slice(observedIntervals, func(i, j int) bool { return observedIntervals[i].start.Before(observedIntervals[j].start) })
	if len(observedIntervals) > 0 {
		maximumGap := observedIntervals[0].start.Sub(manifest.ResearchWindowStart)
		for i := 1; i < len(observedIntervals); i++ {
			if gap := observedIntervals[i].start.Sub(observedIntervals[i-1].end); gap > maximumGap {
				maximumGap = gap
			}
		}
		if gap := manifest.ResearchWindowEnd.Sub(observedIntervals[len(observedIntervals)-1].end); gap > maximumGap {
			maximumGap = gap
		}
		if maximumGap > 0 {
			report.MaximumObservedGapSeconds = int64(maximumGap / time.Second)
		}
	} else if manifest.ResearchWindowEnd.After(manifest.ResearchWindowStart) {
		report.MaximumObservedGapSeconds = int64(manifest.ResearchWindowEnd.Sub(manifest.ResearchWindowStart) / time.Second)
	}
	for _, values := range [][]string{report.ExpectedPartitions, report.ObservedPartitions, report.MissingPartitions, report.DuplicatePartitions, report.OutOfWindowPartitions} {
		sort.Strings(values)
	}
	if len(report.ExpectedPartitions) == 0 {
		report.CoverageRatio = "0.000000"
	} else {
		report.CoverageRatio = fmt.Sprintf("%.6f", float64(len(report.ObservedPartitions))/float64(len(report.ExpectedPartitions)))
	}
	if len(report.MissingPartitions) > 0 || len(report.DuplicatePartitions) > 0 || len(report.OutOfWindowPartitions) > 0 || len(report.ExpectedPartitions) != len(report.ObservedPartitions) {
		report.StrictVerdict = CheckFail
		failures = append(failures, Failure{Code: ReasonCoverageIncomplete, Message: "snapshot set does not satisfy declared coverage policy"})
		return report, failures
	}
	return report, failures
}
