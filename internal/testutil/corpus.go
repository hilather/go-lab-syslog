package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func moduleRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// LoadCorpus returns the committed seed files under testdata/corpus/<name>.
// The directory must exist and contain at least one non-empty file.
func LoadCorpus(t testing.TB, name string) [][]byte {
	t.Helper()
	dir := filepath.Join(moduleRoot(t), "testdata", "corpus", name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("testdata/corpus/%s: %v", name, err)
	}
	var out [][]byte
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out = append(out, body)
	}
	if len(out) == 0 {
		t.Fatalf("testdata/corpus/%s is empty", name)
	}
	return out
}
