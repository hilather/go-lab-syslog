package main

import (
	"bytes"
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
	for _, s := range []string{"version", "help", "validate", "canonicalize", "serve", "healthcheck", "mcp-stdio"} {
		if !strings.Contains(out, s) {
			t.Fatalf("help missing %s: %q", s, out)
		}
	}
}

func TestUnimplementedFailClosed(t *testing.T) {
	for _, cmd := range []string{"validate", "canonicalize", "serve", "healthcheck", "mcp-stdio"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"labsyslog", cmd}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("%s succeeded; unimplemented commands must fail closed", cmd)
		}
		if !strings.Contains(stderr.String(), "not implemented") {
			t.Fatalf("%s stderr %q missing not implemented", cmd, stderr.String())
		}
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
