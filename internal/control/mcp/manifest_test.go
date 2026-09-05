package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestGolden(t *testing.T) {
	want, err := RenderManifest()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repoRoot(t), filepath.FromSlash(ManifestRelPath))
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (run make generate)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run make generate", ManifestRelPath)
	}
	var doc Manifest
	if err := json.Unmarshal(want, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Protocol != ProtocolVersion {
		t.Fatalf("manifest protocol=%q", doc.Protocol)
	}
	if doc.SDK != SDKModule || doc.SDKVersion != SDKVersion {
		t.Fatalf("sdk %s %s", doc.SDK, doc.SDKVersion)
	}
	if len(doc.Tools) == 0 || len(doc.Resources) == 0 {
		t.Fatalf("manifest incomplete: %+v", doc)
	}
	if len(doc.InputSchemas) != len(doc.Tools) {
		t.Fatalf("inputSchemas %d tools %d", len(doc.InputSchemas), len(doc.Tools))
	}
	for _, name := range doc.Tools {
		if _, ok := doc.InputSchemas[name]; !ok {
			t.Errorf("missing inputSchema for %s", name)
		}
	}
}
