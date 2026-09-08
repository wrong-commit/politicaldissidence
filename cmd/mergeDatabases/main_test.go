package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureOutMissing(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "new.json")
	if err := ensureOutMissing(missing); err != nil {
		t.Fatalf("missing path: %v", err)
	}

	existing := filepath.Join(dir, "exists.json")
	if err := os.WriteFile(existing, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureOutMissing(existing); err == nil {
		t.Fatal("expected error when output exists")
	}
}