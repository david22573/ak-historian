package app

import (
	"strings"
	"testing"
)

func TestResearchIdentityManifestCommandExposesOnlyStrictResearchInputs(t *testing.T) {
	for _, name := range []string{
		"data-root", "evidence-root", "out", "manifest-id", "manifest-version",
		"dataset-id", "dataset-version", "source-archive-id", "source-archive",
		"instrument-universe-id", "dataset-start", "dataset-end", "point-in-time-cutoff",
		"availability-policy", "coverage-policy", "pit-evidence-id", "pit-evidence-version",
	} {
		if researchIdentityManifestCmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing strict research identity flag --%s", name)
		}
	}
	for _, flag := range []string{"approved", "promoted", "frozen", "paper-ready", "authorized"} {
		if researchIdentityManifestCmd.Flags().Lookup(flag) != nil {
			t.Fatalf("command exposes governance flag --%s", flag)
		}
	}
	if !strings.Contains(strings.ToLower(researchIdentityManifestCmd.Short), "research") {
		t.Fatalf("command is not explicitly research-only: %q", researchIdentityManifestCmd.Short)
	}
}
