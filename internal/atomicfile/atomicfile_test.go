package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileReplacesCompleteContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("complete-new"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "complete-new" {
		t.Fatalf("content=%q err=%v", got, err)
	}
}

func TestWriteFileRenameFailurePreservesPriorArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(path, []byte("prior"), 0644); err != nil {
		t.Fatal(err)
	}
	ops := systemOps
	ops.rename = func(string, string) error { return errors.New("injected rename failure") }
	if err := writeFile(path, []byte("new"), 0644, ops); err == nil {
		t.Fatal("writeFile() error=nil, want injected failure")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "prior" {
		t.Fatalf("prior artifact changed: content=%q err=%v", got, err)
	}
}
