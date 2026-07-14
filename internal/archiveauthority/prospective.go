package archiveauthority

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ProspectiveReceiptSchemaVersion  = "ak-historian.prospective-ingestion-receipt.v1"
	ProspectiveManifestSchemaVersion = "ak-historian.prospective-manifest.v1"
)

type ProspectiveReceipt struct {
	SchemaVersion               string     `json:"schema_version"`
	DatasetID                   string     `json:"dataset_id"`
	DatasetVersion              string     `json:"dataset_version"`
	SourceSchemaVersion         string     `json:"source_schema_version"`
	AcquisitionTimestamp        time.Time  `json:"acquisition_timestamp"`
	SourceAvailabilityTimestamp *time.Time `json:"source_availability_timestamp"`
	SourceAvailabilityReference string     `json:"source_availability_reference,omitempty"`
	AcquisitionEvidenceType     string     `json:"acquisition_evidence_type"`
	AcquisitionEvidenceHash     string     `json:"acquisition_evidence_hash"`
	ContentHash                 string     `json:"content_hash"`
	ManifestRelativeIdentity    string     `json:"manifest_relative_identity"`
	PartitionKey                string     `json:"partition_key"`
	Symbol                      string     `json:"symbol"`
	CoveredPeriodStart          time.Time  `json:"covered_period_start"`
	CoveredPeriodEnd            time.Time  `json:"covered_period_end"`
	ExpectedPartition           bool       `json:"expected_partition"`
	EvaluationCutoff            time.Time  `json:"evaluation_cutoff"`
	CoveragePolicyVersion       string     `json:"coverage_policy_version"`
	AvailabilityPolicyVersion   string     `json:"availability_policy_version"`
	RegistrationSequence        uint64     `json:"registration_sequence"`
	PreviousReceiptHash         string     `json:"previous_receipt_hash"`
	ReceiptHash                 string     `json:"receipt_hash"`
	LocalStagingPath            string     `json:"-"`
}

type ProspectiveManifest struct {
	SchemaVersion             string               `json:"schema_version"`
	DatasetID                 string               `json:"dataset_id"`
	DatasetVersion            string               `json:"dataset_version"`
	SourceSchemaVersion       string               `json:"source_schema_version"`
	RequiredPrimarySymbols    []string             `json:"required_primary_symbols"`
	RequiredContextSymbols    []string             `json:"required_context_symbols"`
	ExpectedPartitions        []string             `json:"expected_partitions"`
	EvaluationCutoff          time.Time            `json:"evaluation_cutoff"`
	CoveragePolicyVersion     string               `json:"coverage_policy_version"`
	AvailabilityPolicyVersion string               `json:"availability_policy_version"`
	Receipts                  []ProspectiveReceipt `json:"receipts"`
	ManifestHash              string               `json:"manifest_hash"`
}

func stableIdentity(value string) bool {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	return trimmed != "" && !strings.ContainsAny(trimmed, `/\\`) && !strings.Contains(lower, "latest") && !strings.Contains(lower, "current") && !strings.Contains(lower, "mutable")
}

func ValidateProspectiveReceipt(receipt ProspectiveReceipt) error {
	if receipt.SchemaVersion != ProspectiveReceiptSchemaVersion || !stableIdentity(receipt.DatasetID) || !validDigest(receipt.DatasetVersion) || !stableIdentity(receipt.SourceSchemaVersion) {
		return errors.New("prospective receipt identity is missing, mutable, or path-based")
	}
	if receipt.AcquisitionTimestamp.IsZero() || receipt.EvaluationCutoff.IsZero() || receipt.AcquisitionTimestamp.After(receipt.EvaluationCutoff) {
		return errors.New("prospective receipt acquisition/cutoff timestamps are invalid")
	}
	if receipt.SourceAvailabilityTimestamp == nil && strings.TrimSpace(receipt.SourceAvailabilityReference) == "" {
		return errors.New("prospective receipt lacks authoritative source availability")
	}
	if receipt.SourceAvailabilityTimestamp != nil && receipt.SourceAvailabilityTimestamp.After(receipt.AcquisitionTimestamp) {
		return errors.New("source availability cannot follow acquisition")
	}
	if strings.TrimSpace(receipt.AcquisitionEvidenceType) == "" || !validDigest(receipt.AcquisitionEvidenceHash) || !validDigest(receipt.ContentHash) {
		return errors.New("prospective receipt acquisition evidence is incomplete")
	}
	if strings.TrimSpace(receipt.ManifestRelativeIdentity) == "" || strings.HasPrefix(receipt.ManifestRelativeIdentity, "/") || strings.Contains(receipt.ManifestRelativeIdentity, `\`) || strings.Contains(receipt.ManifestRelativeIdentity, "..") || strings.TrimSpace(receipt.PartitionKey) == "" || strings.TrimSpace(receipt.Symbol) == "" {
		return errors.New("prospective receipt manifest-relative identity is invalid")
	}
	if !receipt.CoveredPeriodStart.Before(receipt.CoveredPeriodEnd) || !receipt.ExpectedPartition || strings.TrimSpace(receipt.CoveragePolicyVersion) == "" || strings.TrimSpace(receipt.AvailabilityPolicyVersion) == "" || receipt.RegistrationSequence == 0 {
		return errors.New("prospective receipt coverage declaration is invalid")
	}
	if receipt.RegistrationSequence == 1 {
		if receipt.PreviousReceiptHash != "sha256:"+strings.Repeat("0", 64) {
			return errors.New("first receipt must use the zero previous hash")
		}
	} else if !validDigest(receipt.PreviousReceiptHash) {
		return errors.New("prospective receipt previous hash is invalid")
	}
	return nil
}

func SealProspectiveReceipt(receipt ProspectiveReceipt) (ProspectiveReceipt, error) {
	receipt.SchemaVersion = ProspectiveReceiptSchemaVersion
	receipt.ReceiptHash = ""
	if err := ValidateProspectiveReceipt(receipt); err != nil {
		return ProspectiveReceipt{}, err
	}
	hash, err := canonicalHash(receipt)
	if err != nil {
		return ProspectiveReceipt{}, err
	}
	receipt.ReceiptHash = hash
	return receipt, nil
}

func VerifyProspectiveReceipt(receipt ProspectiveReceipt) error {
	want := receipt.ReceiptHash
	sealed, err := SealProspectiveReceipt(receipt)
	if err != nil {
		return err
	}
	if want != sealed.ReceiptHash {
		return errors.New("prospective receipt hash mismatch")
	}
	return nil
}

func RegisterProspectiveReceipt(existing []ProspectiveReceipt, incoming ProspectiveReceipt) ([]ProspectiveReceipt, bool, error) {
	if err := VerifyProspectiveReceipt(incoming); err != nil {
		return nil, false, err
	}
	result := append([]ProspectiveReceipt{}, existing...)
	for _, receipt := range result {
		if receipt.ManifestRelativeIdentity != incoming.ManifestRelativeIdentity {
			continue
		}
		if receipt.ContentHash == incoming.ContentHash && receipt.ReceiptHash == incoming.ReceiptHash {
			return result, false, nil
		}
		return nil, false, errors.New("conflicting duplicate manifest-relative identity")
	}
	if len(result) > 0 {
		last := result[len(result)-1]
		if incoming.RegistrationSequence != last.RegistrationSequence+1 || incoming.PreviousReceiptHash != last.ReceiptHash {
			return nil, false, errors.New("append-only receipt chain is discontinuous")
		}
	}
	return append(result, incoming), true, nil
}

func SealProspectiveManifest(manifest ProspectiveManifest) (ProspectiveManifest, error) {
	manifest.SchemaVersion = ProspectiveManifestSchemaVersion
	manifest.ManifestHash = ""
	manifest.RequiredPrimarySymbols = sortedUnique(manifest.RequiredPrimarySymbols)
	manifest.RequiredContextSymbols = sortedUnique(manifest.RequiredContextSymbols)
	manifest.ExpectedPartitions = sortedUnique(manifest.ExpectedPartitions)
	manifest.Receipts = append([]ProspectiveReceipt{}, manifest.Receipts...)
	sort.Slice(manifest.Receipts, func(i, j int) bool {
		return manifest.Receipts[i].RegistrationSequence < manifest.Receipts[j].RegistrationSequence
	})
	if !stableIdentity(manifest.DatasetID) || !validDigest(manifest.DatasetVersion) || !stableIdentity(manifest.SourceSchemaVersion) || len(manifest.RequiredPrimarySymbols) == 0 || len(manifest.RequiredContextSymbols) == 0 || len(manifest.ExpectedPartitions) == 0 || manifest.EvaluationCutoff.IsZero() || strings.TrimSpace(manifest.CoveragePolicyVersion) == "" || strings.TrimSpace(manifest.AvailabilityPolicyVersion) == "" {
		return ProspectiveManifest{}, errors.New("prospective manifest authority is incomplete")
	}
	expected := map[string]struct{}{}
	for _, partition := range manifest.ExpectedPartitions {
		expected[partition] = struct{}{}
	}
	covered, symbols := map[string]struct{}{}, map[string]struct{}{}
	previous := "sha256:" + strings.Repeat("0", 64)
	for index, receipt := range manifest.Receipts {
		if err := VerifyProspectiveReceipt(receipt); err != nil {
			return ProspectiveManifest{}, fmt.Errorf("receipt %d: %w", index, err)
		}
		if receipt.DatasetID != manifest.DatasetID || receipt.DatasetVersion != manifest.DatasetVersion || receipt.SourceSchemaVersion != manifest.SourceSchemaVersion || !receipt.EvaluationCutoff.Equal(manifest.EvaluationCutoff) || receipt.CoveragePolicyVersion != manifest.CoveragePolicyVersion || receipt.AvailabilityPolicyVersion != manifest.AvailabilityPolicyVersion || receipt.RegistrationSequence != uint64(index+1) || receipt.PreviousReceiptHash != previous {
			return ProspectiveManifest{}, fmt.Errorf("receipt %d does not bind the manifest authority", index)
		}
		if _, ok := expected[receipt.PartitionKey]; !ok {
			return ProspectiveManifest{}, fmt.Errorf("receipt %d is not an expected partition", index)
		}
		if _, duplicate := covered[receipt.PartitionKey]; duplicate {
			return ProspectiveManifest{}, fmt.Errorf("duplicate expected partition %s", receipt.PartitionKey)
		}
		covered[receipt.PartitionKey], symbols[receipt.Symbol], previous = struct{}{}, struct{}{}, receipt.ReceiptHash
	}
	if len(covered) != len(expected) {
		return ProspectiveManifest{}, errors.New("prospective manifest lacks expected partition coverage")
	}
	for _, symbol := range append(append([]string{}, manifest.RequiredPrimarySymbols...), manifest.RequiredContextSymbols...) {
		if _, ok := symbols[symbol]; !ok {
			return ProspectiveManifest{}, fmt.Errorf("prospective manifest lacks required symbol/context %s", symbol)
		}
	}
	hash, err := canonicalHash(manifest)
	if err != nil {
		return ProspectiveManifest{}, err
	}
	manifest.ManifestHash = hash
	return manifest, nil
}

func VerifyProspectiveManifest(manifest ProspectiveManifest) error {
	want := manifest.ManifestHash
	sealed, err := SealProspectiveManifest(manifest)
	if err != nil {
		return err
	}
	if want != sealed.ManifestHash {
		return errors.New("prospective manifest hash mismatch")
	}
	return nil
}

func sortedUnique(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
