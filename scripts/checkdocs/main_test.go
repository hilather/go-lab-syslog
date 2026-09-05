package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRepoDocuments(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(root); err != nil {
		t.Fatal(err)
	}
}

func TestCheckReportsMissingAndBroken(t *testing.T) {
	dir := t.TempDir()
	if err := Check(dir); err == nil {
		t.Fatal("expected missing documents")
	} else if !strings.Contains(err.Error(), "required documents missing") {
		t.Fatalf("error = %v", err)
	}

	for _, rel := range RequiredRootDocs {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# x\n\nSee [missing](no-such-file.md).\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Check(dir); err == nil {
		t.Fatal("expected broken link")
	} else if !strings.Contains(err.Error(), "broken markdown links") {
		t.Fatalf("error = %v", err)
	}
}

func TestRequiredPhrasesLockCapAdd(t *testing.T) {
	found := false
	for _, p := range RequiredPhrases {
		if p == "cap_add: [NET_BIND_SERVICE]" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("RequiredPhrases must lock cap_add: [NET_BIND_SERVICE] (C17)")
	}
}

func TestCheckReportsMissingPhrases(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range RequiredRootDocs {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Check(dir); err == nil {
		t.Fatal("expected missing phrases")
	} else if !strings.Contains(err.Error(), "required documentation phrases missing") {
		t.Fatalf("error = %v", err)
	}
}
