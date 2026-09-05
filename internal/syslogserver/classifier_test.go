package syslogserver

import (
	"net/netip"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/model"
)

func boolPtr(v bool) *bool { return &v }

func mustClassifier(t *testing.T, filters []model.Filter) *FilterClassifier {
	t.Helper()
	c, err := NewFilterClassifier(filters)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClassifierUnmatchedIsCapture(t *testing.T) {
	c := mustClassifier(t, nil)
	action, tag := c.Classify(&model.Message{RemoteIP: netip.MustParseAddr("10.0.0.1")})
	if action != ActionCapture || tag != "" {
		t.Fatalf("action=%q tag=%q", action, tag)
	}
}

func TestClassifierFirstMatchWinsOverBroaderCIDR(t *testing.T) {
	c := mustClassifier(t, []model.Filter{
		{
			Name:    "drop-host",
			Enabled: boolPtr(true),
			Match:   model.FilterMatch{SourceCIDRs: []string{"10.99.42.1/32"}},
			Action:  model.FilterAction{Mode: ActionDropSilent},
		},
		{
			Name:    "capture-net",
			Enabled: boolPtr(true),
			Match:   model.FilterMatch{SourceCIDRs: []string{"10.99.42.0/24"}},
			Action:  model.FilterAction{Mode: ActionCapture},
		},
	})
	action, _ := c.Classify(&model.Message{RemoteIP: netip.MustParseAddr("10.99.42.1")})
	if action != ActionDropSilent {
		t.Fatalf("narrower first-match action=%q", action)
	}
	action, _ = c.Classify(&model.Message{RemoteIP: netip.MustParseAddr("10.99.42.2")})
	if action != ActionCapture {
		t.Fatalf("broader later action=%q", action)
	}
}

func TestClassifierTag(t *testing.T) {
	c := mustClassifier(t, []model.Filter{
		{
			Name:    "tag-sshd",
			Enabled: boolPtr(true),
			Match:   model.FilterMatch{AppNames: []string{"sshd"}},
			Action:  model.FilterAction{Mode: ActionTag, Tag: "auth"},
		},
	})
	action, tag := c.Classify(&model.Message{Parsed: model.Parsed{AppName: "sshd"}})
	if action != ActionTag || tag != "auth" {
		t.Fatalf("action=%q tag=%q", action, tag)
	}
	action, tag = c.Classify(&model.Message{Parsed: model.Parsed{AppName: "cron"}})
	if action != ActionCapture || tag != "" {
		t.Fatalf("unmatched action=%q tag=%q", action, tag)
	}
}

func TestClassifierDisabledSkipped(t *testing.T) {
	c := mustClassifier(t, []model.Filter{
		{
			Name:    "disabled-drop",
			Enabled: boolPtr(false),
			Match:   model.FilterMatch{SourceCIDRs: []string{"10.0.0.0/8"}},
			Action:  model.FilterAction{Mode: ActionDropSilent},
		},
		{
			Name:    "tag-later",
			Enabled: boolPtr(true),
			Match:   model.FilterMatch{SourceCIDRs: []string{"10.0.0.0/8"}},
			Action:  model.FilterAction{Mode: ActionTag, Tag: "lab"},
		},
	})
	action, tag := c.Classify(&model.Message{RemoteIP: netip.MustParseAddr("10.0.0.9")})
	if action != ActionTag || tag != "lab" {
		t.Fatalf("disabled filter won: action=%q tag=%q", action, tag)
	}
}

func TestClassifierANDPredicates(t *testing.T) {
	c := mustClassifier(t, []model.Filter{
		{
			Name:    "user-debug-udp",
			Enabled: boolPtr(true),
			Match: model.FilterMatch{
				Facilities: []string{"user"},
				Severities: []string{"debug"},
				Transports: []string{TransportUDP},
			},
			Action: model.FilterAction{Mode: ActionDropSilent},
		},
	})
	drop := &model.Message{
		Transport: TransportUDP,
		Parsed:    model.Parsed{Facility: 1, Severity: 7},
	}
	if action, _ := c.Classify(drop); action != ActionDropSilent {
		t.Fatalf("AND match action=%q", action)
	}
	keep := &model.Message{
		Transport: TransportTCP,
		Parsed:    model.Parsed{Facility: 1, Severity: 7},
	}
	if action, _ := c.Classify(keep); action != ActionCapture {
		t.Fatalf("transport miss action=%q", action)
	}
}

func TestClassifierSeverityAtLeast(t *testing.T) {
	c := mustClassifier(t, []model.Filter{
		{
			Name:    "loud",
			Enabled: boolPtr(true),
			Match:   model.FilterMatch{SeverityAtLeast: "err"},
			Action:  model.FilterAction{Mode: ActionTag, Tag: "loud"},
		},
	})
	if action, tag := c.Classify(&model.Message{Parsed: model.Parsed{Severity: 3}}); action != ActionTag || tag != "loud" {
		t.Fatalf("err should match: action=%q tag=%q", action, tag)
	}
	if action, _ := c.Classify(&model.Message{Parsed: model.Parsed{Severity: 4}}); action != ActionCapture {
		t.Fatalf("warning should miss: action=%q", action)
	}
}

func TestClassifierUnmapsFilterCIDR(t *testing.T) {
	c := mustClassifier(t, []model.Filter{
		{
			Name:    "v4",
			Enabled: boolPtr(true),
			Match:   model.FilterMatch{SourceCIDRs: []string{"192.0.2.0/24"}},
			Action:  model.FilterAction{Mode: ActionDropSilent},
		},
	})
	action, _ := c.Classify(&model.Message{RemoteIP: netip.MustParseAddr("::ffff:192.0.2.9")})
	if action != ActionDropSilent {
		t.Fatalf("mapped IPv4 missed filter CIDR: action=%q", action)
	}
}

func TestClassifierInvalidCIDR(t *testing.T) {
	_, err := NewFilterClassifier([]model.Filter{{
		Name:   "bad",
		Match:  model.FilterMatch{SourceCIDRs: []string{"999.0.0.0/8"}},
		Action: model.FilterAction{Mode: ActionCapture},
	}})
	if err == nil {
		t.Fatal("invalid filter CIDR must fail closed")
	}
}
