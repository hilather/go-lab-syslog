package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

func TestCompatValid(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "testdata", "config", "valid")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		n++
		path := filepath.Join(dir, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			doc, err := LoadFile(path)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if err := compiler.Check(doc, filepath.Dir(path)); err != nil {
				t.Fatalf("validate: %v", err)
			}
			canon, err := EncodeCanonical(doc)
			if err != nil {
				t.Fatal(err)
			}
			rev := compiler.Revision(canon)
			if !strings.HasPrefix(rev, "sha256:") || len(rev) != len("sha256:")+64 {
				t.Fatalf("revision %q", rev)
			}
			doc2, err := DecodeYAML(canon)
			if err != nil {
				t.Fatalf("redecode canonical: %v", err)
			}
			if err := compiler.Check(doc2, filepath.Dir(path)); err != nil {
				t.Fatalf("revalidate: %v", err)
			}
			canon2, err := EncodeCanonical(doc2)
			if err != nil {
				t.Fatal(err)
			}
			if string(canon) != string(canon2) {
				t.Fatalf("canonicalize not stable\nfirst:\n%s\nsecond:\n%s", canon, canon2)
			}
			if compiler.Revision(canon2) != rev {
				t.Fatal("revision changed after canonicalize round-trip")
			}
		})
	}
	if n < 3 {
		t.Fatalf("expected at least defaults, lab overlay, maxed; got %d", n)
	}
}

func TestCompatInvalid(t *testing.T) {
	want := map[string]domainerr.Code{
		"unknown-field.yaml":       domainerr.UnknownField,
		"kebab-alias.yaml":         domainerr.UnknownField,
		"tls-enabled.yaml":         domainerr.TLSUnsupported,
		"reserved-forward.yaml":    domainerr.ReservedKey,
		"reserved-remotehost.yaml": domainerr.ReservedKey,
		"reserved-secret.yaml":     domainerr.ReservedKey,
		"empty-cidrs.yaml":         domainerr.ValidationFailed,
		"both-parsers-false.yaml":  domainerr.ValidationFailed,
		"short-token.yaml":         domainerr.ValidationFailed,
		"origin-allowlist.yaml":    domainerr.UnknownField,
		"management-auth.yaml":     domainerr.UnknownField,
		"metrics-listen.yaml":      domainerr.UnknownField,
		"spill-directory.yaml":     domainerr.UnknownField,
		"framing-abbrev.yaml":      domainerr.ValidationFailed,
	}
	dir := filepath.Join(repoRoot(t), "testdata", "config", "invalid")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		code, ok := want[e.Name()]
		if !ok {
			t.Errorf("unexpected invalid fixture %s", e.Name())
			continue
		}
		seen[e.Name()] = true
		path := filepath.Join(dir, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			doc, err := LoadFile(path)
			if err == nil {
				err = compiler.Check(doc, filepath.Dir(path))
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !domainerr.Is(err, code) {
				t.Fatalf("err=%v want code %s", err, code)
			}
		})
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("missing invalid fixture %s", name)
		}
	}
}

func TestMissingTokenFileAllowed(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml")
	doc, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.Check(doc, filepath.Dir(path)); err != nil {
		t.Fatalf("missing token file must be allowed at validate: %v", err)
	}
}

func repoRoot(t *testing.T) string {
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
