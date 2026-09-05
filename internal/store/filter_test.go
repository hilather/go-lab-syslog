package store_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
)

func TestListWaitANDFilters(t *testing.T) {
	s := testStore(t, store.Config{})
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	tru := true
	fal := false

	m1 := model.Message{
		ReceivedAt:   now,
		Transport:    store.TransportUDP,
		RemoteIP:     netip.MustParseAddr("10.99.42.7"),
		Raw:          []byte("one"),
		ParseWarning: "missing_pri",
		Parsed: model.Parsed{
			Facility: 1, Severity: 5, Version: 0,
			Hostname: "alpha", AppName: "sshd", ProcID: "1", MsgID: "id1",
			Message: "accepted user root",
		},
	}
	m2 := model.Message{
		ReceivedAt: now.Add(time.Minute),
		Transport:  store.TransportTCP,
		RemoteIP:   netip.MustParseAddr("192.0.2.9"),
		Raw:        []byte("two"),
		Truncated:  true, // filter coverage; 1.0 ingest never sets this
		Parsed: model.Parsed{
			Facility: 4, Severity: 2, Version: 1,
			Hostname: "beta", AppName: "kernel", ProcID: "0", MsgID: "id2",
			Message: "panic at the disco",
		},
	}
	mustInsert(t, s, m1)
	mustInsert(t, s, m2)

	cases := []struct {
		name string
		f    store.ListFilter
		want string
	}{
		{"facility keyword", store.ListFilter{Facility: "user"}, "accepted user root"},
		{"facility number", store.ListFilter{Facility: "4"}, "panic at the disco"},
		{"severity exact", store.ListFilter{Severity: "crit"}, "panic at the disco"},
		{"severityAtLeast crit includes emerg..crit", store.ListFilter{SeverityAtLeast: "crit"}, "panic at the disco"},
		{"severityAtLeast debug matches notice too", store.ListFilter{SeverityAtLeast: "debug", Transport: store.TransportUDP}, "accepted user root"},
		{"appName", store.ListFilter{AppName: "sshd"}, "accepted user root"},
		{"hostname", store.ListFilter{Hostname: "beta"}, "panic at the disco"},
		{"msgID", store.ListFilter{MsgID: "id2"}, "panic at the disco"},
		{"procID", store.ListFilter{ProcID: "1"}, "accepted user root"},
		{"messageContains", store.ListFilter{MessageContains: "user"}, "accepted user root"},
		{"protocol 3164", store.ListFilter{Protocol: store.ProtocolRFC3164}, "accepted user root"},
		{"protocol 5424", store.ListFilter{Protocol: store.ProtocolRFC5424}, "panic at the disco"},
		{"transport", store.ListFilter{Transport: store.TransportTCP}, "panic at the disco"},
		{"sourceCidr", store.ListFilter{SourceCIDR: "10.99.42.0/24"}, "accepted user root"},
		{"after", store.ListFilter{After: now.Add(30 * time.Second)}, "panic at the disco"},
		{"before", store.ListFilter{Before: now.Add(30 * time.Second)}, "accepted user root"},
		{"truncated true", store.ListFilter{Truncated: &tru}, "panic at the disco"},
		{"truncated false", store.ListFilter{Truncated: &fal}, "accepted user root"},
		{"parseWarning true", store.ListFilter{ParseWarning: &tru}, "accepted user root"},
		{"parseWarning false", store.ListFilter{ParseWarning: &fal}, "panic at the disco"},
		{"AND transport+app", store.ListFilter{Transport: store.TransportUDP, AppName: "sshd"}, "accepted user root"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := s.List(tc.f, "", 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Items) != 1 || res.Items[0].Parsed.Message != tc.want {
				t.Fatalf("got %#v, want %q", bodies(res), tc.want)
			}
			wait, err := s.Wait(t.Context(), tc.f, 10*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			if wait.Matched != store.MatchedExisting || wait.Message.Parsed.Message != tc.want {
				t.Fatalf("wait %+v", wait)
			}
		})
	}

	res, err := s.List(store.ListFilter{AppName: "sshd", Transport: store.TransportTCP}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 0 {
		t.Fatalf("AND miss: %v", bodies(res))
	}

	_, err = s.List(store.ListFilter{Facility: "nope"}, "", 0)
	if !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("bad facility: %v", err)
	}
	_, err = s.List(store.ListFilter{SourceCIDR: "not-a-cidr"}, "", 0)
	if !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("bad cidr: %v", err)
	}
	_, err = s.List(store.ListFilter{Protocol: "rfc666"}, "", 0)
	if !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("bad protocol: %v", err)
	}
}

func TestSeverityAtLeastNumericLE(t *testing.T) {
	s := testStore(t, store.Config{})
	for _, sev := range []uint8{0, 3, 6} {
		m := msg("s")
		m.Parsed.Severity = sev
		m.Parsed.Message = string(rune('0' + sev))
		mustInsert(t, s, m)
	}
	res, err := s.List(store.ListFilter{SeverityAtLeast: "err"}, "", 0) // 3; 0 and 3 match, 6 does not
	if err != nil {
		t.Fatal(err)
	}
	got := bodies(res)
	if len(got) != 2 || got[0] != "3" || got[1] != "0" {
		t.Fatalf("got %v", got)
	}
}

func bodies(res store.ListResult) []string {
	out := make([]string, len(res.Items))
	for i, m := range res.Items {
		out[i] = m.Parsed.Message
	}
	return out
}
