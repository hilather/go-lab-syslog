package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestJSONLoggerLevelAndMutation(t *testing.T) {
	t.Cleanup(func() { SetLevel("info") })
	SetLevel("info")
	var buf bytes.Buffer
	log := NewJSONLogger(&buf)
	log.Info("mutation", "actor", "operator", "operation", "apply", "reason", "test", "revision", "sha256:abc")
	log.Debug("raw body should be dropped at info")
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("lines = %d, body %s", len(lines), buf.String())
	}
	var rec map[string]any
	if err := json.Unmarshal(lines[0], &rec); err != nil {
		t.Fatal(err)
	}
	if rec["msg"] != "mutation" {
		t.Fatalf("msg %v", rec["msg"])
	}
	if rec["actor"] != "operator" || rec["operation"] != "apply" {
		t.Fatalf("record %+v", rec)
	}
	if _, ok := rec["raw"]; ok {
		t.Fatal("must not log raw bodies at info")
	}

	SetLevel("debug")
	buf.Reset()
	log.Debug("datagram", "n", 4)
	if !bytes.Contains(buf.Bytes(), []byte("datagram")) {
		t.Fatalf("debug missing: %s", buf.String())
	}
}

func TestParseLevel(t *testing.T) {
	if ParseLevel("warn") != slog.LevelWarn {
		t.Fatal("warn")
	}
	if ParseLevel("DEBUG") != slog.LevelDebug {
		t.Fatal("debug")
	}
	if ParseLevel("nope") != slog.LevelInfo {
		t.Fatal("default info")
	}
}
