package researchidentity

import (
	"fmt"
	"os"

	"github.com/david22573/ak-historian/internal/canonicalcontract"
)

func hashFile(path, role string) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", 0, err
	}
	if !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("%s is not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	hash, err := canonicalcontract.HashRawReader(RawObjectContractName, 1, role, info.Size(), f)
	if err != nil {
		return "", 0, err
	}
	return hash, info.Size(), nil
}

func computeDatasetHash(dataset DatasetIdentity) (string, error) {
	hash, _, err := canonicalcontract.HashArtifactValue(DatasetSchemaName, ManifestSchemaVersion, DatasetArtifactRole, "artifact_hash", dataset)
	return hash, err
}

func computeArchiveHash(archive ArchiveIdentity) (string, error) {
	hash, _, err := canonicalcontract.HashArtifactValue(ArchiveSchemaName, ManifestSchemaVersion, ArchiveArtifactRole, "artifact_hash", archive)
	return hash, err
}

func computeCoverageHash(coverage CoverageEvidence) (string, error) {
	hash, _, err := canonicalcontract.HashArtifactValue(CoverageSchemaName, ManifestSchemaVersion, CoverageArtifactRole, "artifact_hash", coverage)
	return hash, err
}

func computePITEvidenceHash(pit PITEvidence) (string, error) {
	hash, _, err := canonicalcontract.HashArtifactValue(PITSchemaName, ManifestSchemaVersion, PITArtifactRole, "artifact_hash", pit)
	return hash, err
}

func computeManifestHash(manifest Manifest) (string, error) {
	hash, _, err := canonicalcontract.HashArtifactValue(ManifestSchemaName, ManifestSchemaVersion, ManifestArtifactRole, "artifact_hash", manifest)
	return hash, err
}

func computeAvailabilityPolicyHash(policy AvailabilityPolicy) (string, error) {
	hash, _, err := canonicalcontract.HashArtifactValue(AvailabilitySchemaName, ManifestSchemaVersion, AvailabilityArtifactRole, "artifact_hash", policy)
	return hash, err
}

func computeCoveragePolicyHash(policy CoveragePolicy) (string, error) {
	hash, _, err := canonicalcontract.HashArtifactValue(CoveragePolicySchemaName, ManifestSchemaVersion, CoveragePolicyArtifactRole, "artifact_hash", policy)
	return hash, err
}
