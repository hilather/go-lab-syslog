package syslogwire

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

func TestLooksLikeRFC5424(t *testing.T) {
	yes := []string{
		"<1>1 ",
		"<13>1 2026-09-04T20:52:35Z host app - - -",
		"<165>1 2026-09-04T20:52:35Z h a - - -",
		"<191>1 ",
		"<200>1 x",
	}
	for _, s := range yes {
		if !looksLikeRFC5424([]byte(s)) {
			t.Errorf("looksLikeRFC5424(%q) = false", s)
		}
	}
	no := []string{
		"",
		"<13>",
		"<13>1",
		"<13>2 ",
		"<13>Sep  4 20:52:35 host app: x",
		"<9999>1 ",
		"1 2026-09-04T20:52:35Z host app - - -",
		"<13>10 2026-09-04T20:52:35Z host app - - -",
	}
	for _, s := range no {
		if looksLikeRFC5424([]byte(s)) {
			t.Errorf("looksLikeRFC5424(%q) = true", s)
		}
	}
}

func TestRFC5424OnlyParser(t *testing.T) {
	raw := []byte("<13>1 2026-09-04T20:52:35Z host app - - - hi")
	p, warn, err := Parse(raw, Options{RFC3164: false, RFC5424: true, BestEffort: false, Now: goldenNow()})
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" || p.Version != 1 || p.Message != "hi" {
		t.Fatalf("warn=%q parsed=%+v", warn, p)
	}
}

func TestProtocolSelectionRFC5424Wins(t *testing.T) {
	raw := []byte("<13>1 2026-09-04T20:52:35Z host app - - - hi")
	p, warn, err := Parse(raw, Options{RFC3164: true, RFC5424: true, BestEffort: true, Now: goldenNow()})
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warning = %q", warn)
	}
	if p.Version != 1 {
		t.Fatalf("version = %d, want 1", p.Version)
	}
	if p.Message != "hi" {
		t.Fatalf("message = %q", p.Message)
	}
}

func TestProtocolSelectionNoParserDrop(t *testing.T) {
	raw := []byte("<34>Sep  4 20:52:35 sut-1 sshd[1234]: Failed password")
	_, _, err := Parse(raw, Options{RFC3164: false, RFC5424: true, BestEffort: false, Now: goldenNow()})
	if !domainerr.Is(err, domainerr.Unparseable) {
		t.Fatalf("err = %v, want unparseable", err)
	}
}

func TestYearNotRolledBackWithin24h(t *testing.T) {
	now := time.Date(2026, 9, 4, 20, 0, 0, 0, time.UTC)
	raw := []byte("<13>Sep  5 19:00:00 sut-1 logger: skew")
	p, warn, err := Parse(raw, Options{RFC3164: true, RFC5424: true, BestEffort: true, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warning = %q", warn)
	}
	want := time.Date(2026, 9, 5, 19, 0, 0, 0, time.UTC)
	if !p.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %s, want %s", p.Timestamp, want)
	}
}

func TestYearRolledBackMoreThan24h(t *testing.T) {
	now := time.Date(2026, 9, 4, 20, 0, 0, 0, time.UTC)
	raw := []byte("<13>Sep  5 21:00:00 sut-1 logger: future")
	p, _, err := Parse(raw, Options{RFC3164: true, RFC5424: true, BestEffort: true, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2025, 9, 5, 21, 0, 0, 0, time.UTC)
	if !p.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %s, want %s", p.Timestamp, want)
	}
}

func TestOptionsFromParseNilDefaults(t *testing.T) {
	o := OptionsFromParse(model.SyslogParse{})
	if !o.RFC3164 || !o.RFC5424 || !o.BestEffort {
		t.Fatalf("%+v", o)
	}
	f := false
	o = OptionsFromParse(model.SyslogParse{RFC5424: &f, BestEffort: &f, RFC3164: &f})
	if o.RFC3164 || o.RFC5424 || o.BestEffort {
		t.Fatalf("%+v", o)
	}
}

func TestBestEffortEmpty(t *testing.T) {
	p, warn, err := Parse(nil, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if warn != WarnMissingPRI {
		t.Fatalf("warning = %q", warn)
	}
	if p.PRI != defaultPRI || p.Version != 0 {
		t.Fatalf("%+v", p)
	}
}

func TestUnknownFacilityNotDroppedWhenStrict(t *testing.T) {
	raw := []byte("<200>Sep  4 20:52:35 sut-1 app: hi")
	p, warn, err := Parse(raw, Options{RFC3164: true, RFC5424: true, BestEffort: false, Now: goldenNow()})
	if err != nil {
		t.Fatal(err)
	}
	if warn != WarnUnknownFacility {
		t.Fatalf("warning = %q", warn)
	}
	if p.PRI != 200 || p.Facility != 25 || p.Severity != 0 {
		t.Fatalf("%+v", p)
	}

	raw5424 := []byte("<200>1 2026-09-04T20:52:35Z sut-1 app - - - hi")
	p, warn, err = Parse(raw5424, Options{RFC3164: true, RFC5424: true, BestEffort: false, Now: goldenNow()})
	if err != nil {
		t.Fatal(err)
	}
	if warn != WarnUnknownFacility || p.PRI != 200 || p.Version != 1 || p.Message != "hi" {
		t.Fatalf("warn=%q parsed=%+v", warn, p)
	}
}

func TestRFC5424IncompleteStrict(t *testing.T) {
	_, _, err := Parse([]byte("<13>1 2026-09-04T20:52:35Z only-host"), Options{
		RFC5424: true, BestEffort: false, Now: goldenNow(),
	})
	if !domainerr.Is(err, domainerr.Unparseable) {
		t.Fatalf("err = %v", err)
	}
}

func TestRFC5424IncompleteBestEffort(t *testing.T) {
	p, warn, err := Parse([]byte("<13>1 2026-09-04T20:52:35Z only-host"), Options{
		RFC5424: true, BestEffort: true, Now: goldenNow(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warning = %q", warn)
	}
	if p.Version != 1 || p.Hostname != "only-host" {
		t.Fatalf("%+v", p)
	}
}

func goldenNow() time.Time {
	return time.Date(2026, 9, 4, 20, 52, 35, 0, time.UTC)
}
