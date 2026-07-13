package pitarchive

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicAuthoritativeWrites(t *testing.T) {
	t.Run("manifest and evidence are not world writable", func(t *testing.T) {
		fixture := newPITFixture(t)
		manifestOutput := filepath.Join(t.TempDir(), "manifest.json")
		if err := WriteManifest(manifestOutput, fixture.manifest); err != nil {
			t.Fatal(err)
		}
		evidence := validEvidence(t)
		evidenceOutput := filepath.Join(t.TempDir(), "evidence.json")
		if err := WriteEvidence(evidenceOutput, evidence); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{manifestOutput, evidenceOutput} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm()&0o002 != 0 {
				t.Fatalf("authoritative artifact is world writable: %s %o", path, info.Mode().Perm())
			}
		}
	})

	t.Run("failed write preserves prior artifact", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "evidence.json")
		prior := []byte("prior-valid-artifact")
		if err := os.WriteFile(path, prior, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := writeAtomic(path, []byte("replacement-too-large"), 1); err == nil {
			t.Fatal("oversized atomic write unexpectedly succeeded")
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(after, prior) {
			t.Fatalf("failed write changed prior artifact: %q", after)
		}
	})

	t.Run("temporary files are removed after success", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "evidence.json")
		if err := writeAtomic(path, []byte("complete"), 100); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "evidence.json" {
			t.Fatalf("unexpected atomic-write residue: %+v", entries)
		}
	})
}
