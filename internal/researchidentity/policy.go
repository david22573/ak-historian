package researchidentity

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/david22573/ak-historian/internal/canonicalcontract"
)

func loadAvailabilityPolicy(path string) (AvailabilityPolicy, RawObjectReference, error) {
	var policy AvailabilityPolicy
	data, err := readAndValidateArtifact(path, AvailabilitySchemaName, &policy)
	if err != nil {
		return policy, RawObjectReference{}, fmt.Errorf("availability policy: %w", err)
	}
	if err := validateIdentityText("availability policy id", policy.PolicyID); err != nil {
		return policy, RawObjectReference{}, err
	}
	if err := validateIdentityText("availability policy version", policy.PolicyVersion); err != nil {
		return policy, RawObjectReference{}, err
	}
	if policy.AvailabilityDelayNS < 0 || policy.AvailabilityDelayNS > int64(365*24*time.Hour) {
		return policy, RawObjectReference{}, fmt.Errorf("availability delay is outside the supported bound")
	}
	rawHash, err := canonicalcontract.HashRaw(RawObjectContractName, 1, "availability_policy_source", data)
	if err != nil {
		return policy, RawObjectReference{}, err
	}
	return policy, RawObjectReference{
		LogicalID:  policy.PolicyID,
		MediaType:  "application/json",
		ObjectHash: rawHash,
		SizeBytes:  int64(len(data)),
	}, nil
}

func loadCoveragePolicy(path string) (CoveragePolicy, RawObjectReference, error) {
	var policy CoveragePolicy
	data, err := readAndValidateArtifact(path, CoveragePolicySchemaName, &policy)
	if err != nil {
		return policy, RawObjectReference{}, fmt.Errorf("coverage policy: %w", err)
	}
	if err := validateIdentityText("coverage policy id", policy.PolicyID); err != nil {
		return policy, RawObjectReference{}, err
	}
	if err := validateIdentityText("coverage policy version", policy.PolicyVersion); err != nil {
		return policy, RawObjectReference{}, err
	}
	if policy.Mode != StrictCoverageMode || policy.IntervalNS <= 0 || policy.AllowGaps || policy.AllowDuplicates || policy.AllowOutOfOrder || policy.AllowPartialWindow {
		return policy, RawObjectReference{}, fmt.Errorf("coverage policy must be strict zero-defect full-window")
	}
	rawHash, err := canonicalcontract.HashRaw(RawObjectContractName, 1, "coverage_policy_source", data)
	if err != nil {
		return policy, RawObjectReference{}, err
	}
	return policy, RawObjectReference{
		LogicalID:  policy.PolicyID,
		MediaType:  "application/json",
		ObjectHash: rawHash,
		SizeBytes:  int64(len(data)),
	}, nil
}

func readAndValidateArtifact(path, expectedSchema string, target any) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result, err := canonicalcontract.ValidateArtifact(data, true)
	if err != nil {
		return nil, err
	}
	if result.SchemaName != expectedSchema {
		return nil, fmt.Errorf("schema mismatch: expected %s, got %s", expectedSchema, result.SchemaName)
	}
	if err := json.Unmarshal(result.CanonicalBytes, target); err != nil {
		return nil, fmt.Errorf("decode validated artifact: %w", err)
	}
	return data, nil
}
