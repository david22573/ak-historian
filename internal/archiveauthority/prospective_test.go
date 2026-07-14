package archiveauthority

import (
	"strings"
	"testing"
	"time"
)

func TestProspectiveReceiptAndManifestContracts(t *testing.T) {
	receipt := validProspectiveReceipt("BTCUSDT", "p-btc", 1, "sha256:"+strings.Repeat("0", 64))
	sealed, err := SealProspectiveReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyProspectiveReceipt(sealed); err != nil {
		t.Fatal(err)
	}

	missingAvailability := receipt
	missingAvailability.SourceAvailabilityTimestamp, missingAvailability.SourceAvailabilityReference = nil, ""
	if _, err := SealProspectiveReceipt(missingAvailability); err == nil {
		t.Fatal("missing source availability became PIT eligible")
	}
	missingSchema := receipt
	missingSchema.SourceSchemaVersion = ""
	if _, err := SealProspectiveReceipt(missingSchema); err == nil {
		t.Fatal("missing schema identity passed")
	}
	for _, id := range []string{"latest", "local/path", "/tmp/dataset"} {
		mutable := receipt
		mutable.DatasetID = id
		if _, err := SealProspectiveReceipt(mutable); err == nil {
			t.Fatalf("mutable/path identity %q passed", id)
		}
	}

	left := receipt
	left.LocalStagingPath = "/tmp/one"
	right := receipt
	right.LocalStagingPath = "/different/local/path"
	leftSealed, _ := SealProspectiveReceipt(left)
	rightSealed, _ := SealProspectiveReceipt(right)
	if leftSealed.ReceiptHash != rightSealed.ReceiptHash {
		t.Fatal("local path affected receipt identity")
	}

	registered, added, err := RegisterProspectiveReceipt(nil, sealed)
	if err != nil || !added || len(registered) != 1 {
		t.Fatal("synthetic acquisition did not get a receipt")
	}
	registered, added, err = RegisterProspectiveReceipt(registered, sealed)
	if err != nil || added || len(registered) != 1 {
		t.Fatal("identical duplicate was not idempotent")
	}
	conflict := sealed
	conflict.ContentHash = testDigest('f')
	conflict.ReceiptHash = ""
	conflict, _ = SealProspectiveReceipt(conflict)
	if _, _, err := RegisterProspectiveReceipt(registered, conflict); err == nil {
		t.Fatal("conflicting duplicate content passed")
	}

	eth := validProspectiveReceipt("ETHUSDT", "p-eth", 2, sealed.ReceiptHash)
	eth, err = SealProspectiveReceipt(eth)
	if err != nil {
		t.Fatal(err)
	}
	manifest := ProspectiveManifest{DatasetID: receipt.DatasetID, DatasetVersion: receipt.DatasetVersion, SourceSchemaVersion: receipt.SourceSchemaVersion, RequiredPrimarySymbols: []string{"BTCUSDT"}, RequiredContextSymbols: []string{"ETHUSDT"}, ExpectedPartitions: []string{"p-btc", "p-eth"}, EvaluationCutoff: receipt.EvaluationCutoff, CoveragePolicyVersion: receipt.CoveragePolicyVersion, AvailabilityPolicyVersion: receipt.AvailabilityPolicyVersion, Receipts: []ProspectiveReceipt{sealed, eth}}
	manifest, err = SealProspectiveManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyProspectiveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	missingContext := manifest
	missingContext.Receipts = missingContext.Receipts[:1]
	missingContext.ExpectedPartitions = []string{"p-btc"}
	missingContext.ManifestHash = ""
	if _, err := SealProspectiveManifest(missingContext); err == nil {
		t.Fatal("missing context coverage passed")
	}
}

func validProspectiveReceipt(symbol, partition string, sequence uint64, previous string) ProspectiveReceipt {
	start := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	available, acquired, cutoff := start.AddDate(0, 1, 0), start.AddDate(0, 1, 0).Add(time.Hour), start.AddDate(0, 1, 0).Add(2*time.Hour)
	return ProspectiveReceipt{DatasetID: "synthetic-future-dataset-v1", DatasetVersion: testDigest('a'), SourceSchemaVersion: SourceCandleSchemaVersion, AcquisitionTimestamp: acquired, SourceAvailabilityTimestamp: &available, AcquisitionEvidenceType: "SYNTHETIC_SIGNED_RECEIPT", AcquisitionEvidenceHash: testDigest('b'), ContentHash: testDigest('c'), ManifestRelativeIdentity: "candles/" + symbol + "-2030-01.parquet", PartitionKey: partition, Symbol: symbol, CoveredPeriodStart: start, CoveredPeriodEnd: start.AddDate(0, 1, 0), ExpectedPartition: true, EvaluationCutoff: cutoff, CoveragePolicyVersion: "coverage-v1", AvailabilityPolicyVersion: "availability-v1", RegistrationSequence: sequence, PreviousReceiptHash: previous}
}
