package researchidentity

import (
	"testing"

	"github.com/david22573/ak-historian/internal/canonicalcontract"
)

func TestCanonicalHashDomainBindsContractVersionProfileAndRole(t *testing.T) {
	payload := []byte(`{"flag":true,"negative":-2,"text":"abc"}`)
	got, err := canonicalcontract.HashCanonical("ak.test.identity_primitive", 1, canonicalcontract.CanonicalJSONVersion, "artifact", payload)
	if err != nil {
		t.Fatal(err)
	}
	const want = "sha256:3d02bce09846db16bc9e1bf100b38fc8c20a4e80e72662ffa5b6dd0c187f0f40"
	if got != want {
		t.Fatalf("canonical primitive vector = %s, want %s", got, want)
	}
	changedRole, err := canonicalcontract.HashCanonical("ak.test.identity_primitive", 1, canonicalcontract.CanonicalJSONVersion, "different_role", payload)
	if err != nil {
		t.Fatal(err)
	}
	if changedRole == got {
		t.Fatal("artifact role did not separate the hash domain")
	}
}
