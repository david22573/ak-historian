package pitarchive

import "testing"

func validEvidence(t *testing.T) EvidenceEnvelope {
	t.Helper()
	fixture := newPITFixture(t)
	result := evaluateFixture(t, fixture)
	if result.Evidence == nil || !result.StrictPromotionAllowed {
		t.Fatalf("valid fixture did not produce strict evidence: %+v", result)
	}
	return *result.Evidence
}

func requireEvidenceReason(t *testing.T, evidence EvidenceEnvelope, code ReasonCode) {
	t.Helper()
	for _, failure := range VerifyEvidence(evidence) {
		if failure.Code == code {
			return
		}
	}
	t.Fatalf("evidence verification did not return %s", code)
}

func rehashEvidence(t *testing.T, evidence *EvidenceEnvelope) {
	t.Helper()
	hash, err := ComputeEvidenceIntegrityHash(*evidence)
	if err != nil {
		t.Fatal(err)
	}
	evidence.IntegrityHash = hash
}

func TestEvidenceBinding(t *testing.T) {
	t.Run("valid evidence verifies", func(t *testing.T) {
		evidence := validEvidence(t)
		if failures := VerifyEvidence(evidence); len(failures) != 0 {
			t.Fatalf("valid evidence failed: %+v", failures)
		}
	})

	mutations := []struct {
		name   string
		mutate func(*EvidenceEnvelope)
	}{
		{"dataset mutation", func(e *EvidenceEnvelope) { e.DatasetID = "mutated-dataset" }},
		{"window mutation", func(e *EvidenceEnvelope) { e.ResearchWindowEnd = e.ResearchWindowEnd.Add(-1) }},
		{"manifest hash mutation", func(e *EvidenceEnvelope) {
			e.ManifestHash = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"snapshot set mutation", func(e *EvidenceEnvelope) {
			e.SnapshotSetDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{"coverage verdict mutation", func(e *EvidenceEnvelope) { e.Coverage.StrictVerdict = CheckFail }},
		{"availability verdict mutation", func(e *EvidenceEnvelope) { e.Availability.StrictVerdict = CheckFail }},
		{"final verdict mutation", func(e *EvidenceEnvelope) { e.FinalVerdict = VerdictIneligible }},
	}
	for _, test := range mutations {
		t.Run(test.name+" invalidates evidence", func(t *testing.T) {
			evidence := validEvidence(t)
			test.mutate(&evidence)
			requireEvidenceReason(t, evidence, ReasonEvidenceIntegrityHashMismatch)
		})
	}

	t.Run("unknown evidence schema fails", func(t *testing.T) {
		evidence := validEvidence(t)
		evidence.SchemaVersion = "ak-historian.pit-evidence.v999"
		rehashEvidence(t, &evidence)
		requireEvidenceReason(t, evidence, ReasonEvidenceSchemaUnsupported)
	})

	t.Run("strict approval with noneligible verdict fails", func(t *testing.T) {
		evidence := validEvidence(t)
		evidence.FinalVerdict = VerdictIneligible
		evidence.StrictPromotionAllowed = true
		rehashEvidence(t, &evidence)
		requireEvidenceReason(t, evidence, ReasonEvidenceStrictPromotionInvalid)
	})
}
