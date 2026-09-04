package syslogwire

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
)

func TestSerializeRoundTripRFC5424(t *testing.T) {
	raw := []byte(`<165>1 2026-09-04T20:52:35Z sut-1 sshd 1234 ID47 [sshd@0 user="alice"] Failed password`)
	p, warn, err := Parse(raw, Options{RFC5424: true, BestEffort: false, Now: goldenNow()})
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warning = %q", warn)
	}
	out := Serialize(p)
	p2, warn, err := Parse(out, Options{RFC5424: true, BestEffort: false, Now: goldenNow()})
	if err != nil {
		t.Fatalf("reparse %q: %v", out, err)
	}
	if warn != "" {
		t.Fatalf("warning = %q", warn)
	}
	assertParsedEqual(t, p2, p)
}

func TestSerializeRoundTripRFC5424Escapes(t *testing.T) {
	p := model.Parsed{
		PRI: 13, Facility: 1, Severity: 5, Version: 1,
		Timestamp: time.Date(2026, 9, 4, 20, 52, 35, 0, time.UTC),
		Hostname:  "h",
		AppName:   "a",
		Structured: []model.SDElement{{
			ID:     "ex",
			Params: []model.SDParam{{Name: "p", Value: `quote=" slash\ bracket]`}},
		}},
		Message: "msg",
	}
	out := Serialize(p)
	p2, warn, err := Parse(out, Options{RFC5424: true, BestEffort: false, Now: goldenNow()})
	if err != nil {
		t.Fatalf("reparse %q: %v", out, err)
	}
	if warn != "" {
		t.Fatalf("warning = %q", warn)
	}
	assertParsedEqual(t, p2, p)
}

func TestSerializeRoundTripRFC3164(t *testing.T) {
	raw := []byte("<34>Sep  4 20:52:35 sut-1 sshd[1234]: Failed password")
	p, warn, err := Parse(raw, Options{RFC3164: true, BestEffort: false, Now: goldenNow()})
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warning = %q", warn)
	}
	out := Serialize(p)
	if string(out) != string(raw) {
		t.Fatalf("serialize = %q, want %q", out, raw)
	}
	p2, warn, err := Parse(out, Options{RFC3164: true, BestEffort: false, Now: goldenNow()})
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warning = %q", warn)
	}
	assertParsedEqual(t, p2, p)
}

func TestSerializeNILVALUE(t *testing.T) {
	p := model.Parsed{PRI: 13, Facility: 1, Severity: 5, Version: 1}
	got := string(Serialize(p))
	want := "<13>1 - - - - - -"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSerializeBOMNotReinserted(t *testing.T) {
	p := model.Parsed{
		PRI: 13, Facility: 1, Severity: 5, Version: 1,
		Timestamp: time.Date(2026, 9, 4, 20, 52, 35, 0, time.UTC),
		Hostname:  "host",
		AppName:   "app",
		Message:   "hello",
	}
	got := string(Serialize(p))
	if got != "<13>1 2026-09-04T20:52:35Z host app - - - hello" {
		t.Fatalf("got %q", got)
	}
}
