package pitarchive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionedCompatibilityEvidenceFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "pitarchive", "v1", "evidence.example.json")
	if os.Getenv("UPDATE_PIT_COMPATIBILITY_FIXTURE") == "1" {
		evidence := validEvidence(t)
		data, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read compatibility fixture: %v", err)
	}
	var evidence EvidenceEnvelope
	if err := json.Unmarshal(data, &evidence); err != nil {
		t.Fatalf("decode compatibility fixture: %v", err)
	}
	if failures := VerifyEvidence(evidence); len(failures) != 0 {
		t.Fatalf("compatibility fixture failed verification: %+v", failures)
	}
}
