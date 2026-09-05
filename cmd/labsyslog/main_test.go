package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"labsyslog", "labsyslog.dev/v1alpha1", "2026-07-28", "commit=", "built="} {
		if !strings.Contains(out, want) {
			t.Fatalf("version output %q missing %q", out, want)
		}
	}
}

func TestUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr %q missing usage", stderr.String())
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	for _, s := range []string{"version", "help", "validate", "canonicalize", "serve", "healthcheck", "mcp-stdio", "receive-only", "no send command"} {
		if !strings.Contains(out, s) {
			t.Fatalf("help missing %s: %q", s, out)
		}
	}
}

func TestHealthcheckRequiresURL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "healthcheck"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--url") {
		t.Fatalf("stderr %q missing --url", stderr.String())
	}
}

func TestMCPStdioRequiresFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "mcp-stdio"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--config") {
		t.Fatalf("stderr %q missing --config", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"labsyslog", "mcp-stdio", "--config", "x.yaml"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--token-file") {
		t.Fatalf("stderr %q missing --token-file", stderr.String())
	}
}

func TestSendForbidden(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "send"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("send succeeded")
	}
	if !strings.Contains(stderr.String(), "forbidden") {
		t.Fatalf("stderr %q missing forbidden", stderr.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "nope"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr %q missing unknown command", stderr.String())
	}
}

func TestValidateDefaults(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "validate", "--config", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "revision: sha256:") {
		t.Fatalf("stdout %q missing revision", stdout.String())
	}
}

func TestValidateMissingTokenFile(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "validate", "--config", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("missing token file must exit 0, got %d stderr=%q", code, stderr.String())
	}
}

func TestValidateInvalid(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata", "config", "invalid", "tls-enabled.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "validate", "--config", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "tls_unsupported") {
		t.Fatalf("stderr %q missing tls_unsupported", stderr.String())
	}
}

func TestCanonicalize(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "canonicalize", "--config", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "apiVersion: labsyslog.dev/v1alpha1") {
		t.Fatalf("stdout %q missing document", stdout.String())
	}
	if !strings.Contains(stderr.String(), "revision: sha256:") {
		t.Fatalf("stderr %q missing revision", stderr.String())
	}
	var stdout2, stderr2 bytes.Buffer
	code = run([]string{"labsyslog", "canonicalize", "--config", path}, &stdout2, &stderr2)
	if code != 0 {
		t.Fatal(stderr2.String())
	}
	if stdout.String() != stdout2.String() || stderr.String() != stderr2.String() {
		t.Fatal("canonicalize not stable")
	}
}

func TestValidateRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2", code)
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
