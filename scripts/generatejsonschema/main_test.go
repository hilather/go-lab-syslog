package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderValidJSON(t *testing.T) {
	b, err := render()
	if err != nil {
		t.Fatal(err)
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	m := raw.(map[string]any)
	if m["title"] != "LabSyslog" {
		t.Fatalf("title %v", m["title"])
	}
	if m["additionalProperties"] != false {
		t.Fatal("top-level additionalProperties must be false")
	}
}

func TestCheckFailsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir); err == nil {
		t.Fatal("expected missing schema")
	}
}

func TestGenerateThenCheck(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir); err != nil {
		t.Fatal(err)
	}
}
