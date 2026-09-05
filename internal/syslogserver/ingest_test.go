package syslogserver

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
)

func testPipeline(h Handler, cfg Config) *Server {
	cfg.Handler = h
	cfg = applyDefaults(cfg)
	return &Server{cfg: cfg, metrics: cfg.Metrics, ctx: context.Background()}
}

func TestIngestParseInsidePipeline(t *testing.T) {
	h := newFakeHandler()
	s := testPipeline(h, Config{Parse: syslogwire.DefaultOptions()})
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("127.0.0.1"), port: 1234}, []byte("<165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 ID47 - hi"))
	m := waitMsg(t, h)
	if m.Parsed.Version != 1 || m.Parsed.PRI != 165 {
		t.Fatalf("parsed=%+v", m.Parsed)
	}
	if m.Truncated {
		t.Fatal("truncated")
	}
	if m.ParseWarning != "" {
		t.Fatalf("warning = %q", m.ParseWarning)
	}
}

func TestUnknownFacilityNotDropped(t *testing.T) {
	h := newFakeHandler()
	s := testPipeline(h, Config{Parse: syslogwire.Options{RFC3164: true, RFC5424: true, BestEffort: false}})
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("127.0.0.1"), port: 1}, []byte("<200>Sep  4 20:52:35 host app: x"))
	m := waitMsg(t, h)
	if m.ParseWarning != syslogwire.WarnUnknownFacility {
		t.Fatalf("warning = %q", m.ParseWarning)
	}
	if len(h.snapshot()) != 1 {
		t.Fatal("unknown_facility must be stored, not dropped")
	}
}

func TestBehaviorDropSilentSkipsParse(t *testing.T) {
	h := newFakeHandler()
	s := testPipeline(h, Config{Behavior: Behavior{Mode: BehaviorDropSilent}})
	if s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("127.0.0.1"), port: 1}, []byte(helloPayload)) {
		t.Fatal("drop-silent must not close TCP")
	}
	if s.metrics.DroppedBehavior.Load() != 1 {
		t.Fatalf("behavior drops = %d", s.metrics.DroppedBehavior.Load())
	}
	if len(h.snapshot()) != 0 {
		t.Fatal("behavior drop stored a message")
	}
}

func TestBehaviorCloseSignalsTCPClose(t *testing.T) {
	h := newFakeHandler()
	s := testPipeline(h, Config{Behavior: Behavior{Mode: BehaviorClose}})
	if !s.ingest(context.Background(), TransportTCP, remoteAddr{ip: netip.MustParseAddr("127.0.0.1"), port: 1}, []byte(helloPayload)) {
		t.Fatal("behavior close must close TCP")
	}
	if len(h.snapshot()) != 0 {
		t.Fatal("behavior close stored a message")
	}
}

type denyReason struct{ reason string }

func (d denyReason) Allow(netip.Addr) (bool, string) { return false, d.reason }

func TestAdmissionDenySkipsHandler(t *testing.T) {
	h := newFakeHandler()
	s := testPipeline(h, Config{Admission: denyReason{}})
	if !s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("10.0.0.1"), port: 1}, []byte(helloPayload)) {
		t.Fatal("admission deny must close TCP")
	}
	if s.metrics.DroppedAdmission.Load() != 1 {
		t.Fatalf("admission drops = %d", s.metrics.DroppedAdmission.Load())
	}
	if s.metrics.DroppedAdmissionRate.Load() != 0 {
		t.Fatal("empty Allow reason must not count as admission_rate")
	}
	if s.metrics.Received.Load() != 0 {
		t.Fatal("received counts pre-admission")
	}
	if len(h.snapshot()) != 0 {
		t.Fatal("denied message stored")
	}
}

func TestAdmissionRateReasonIsNotCIDR(t *testing.T) {
	h := newFakeHandler()
	s := testPipeline(h, Config{Admission: denyReason{reason: ReasonAdmissionRate}})
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("10.0.0.1"), port: 1}, []byte(helloPayload))
	if s.metrics.DroppedAdmissionRate.Load() != 1 {
		t.Fatalf("rate drops = %d", s.metrics.DroppedAdmissionRate.Load())
	}
	if s.metrics.DroppedAdmission.Load() != 0 {
		t.Fatal("admission_rate counted as admission_cidr")
	}
	if len(h.snapshot()) != 0 {
		t.Fatal("rate-denied message stored")
	}
}

func TestUnknownAdmissionReasonMapsToCIDR(t *testing.T) {
	h := newFakeHandler()
	s := testPipeline(h, Config{Admission: denyReason{reason: "nope"}})
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("10.0.0.1"), port: 1}, []byte(helloPayload))
	if s.metrics.DroppedAdmission.Load() != 1 {
		t.Fatalf("unknown Allow reason should map to admission_cidr, got cidr=%d rate=%d",
			s.metrics.DroppedAdmission.Load(), s.metrics.DroppedAdmissionRate.Load())
	}
}

type dropAll struct{}

func (dropAll) Classify(*model.Message) (string, string) { return ActionDropSilent, "" }

func TestClassifierDropSilent(t *testing.T) {
	h := newFakeHandler()
	s := testPipeline(h, Config{Classifier: dropAll{}})
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("127.0.0.1"), port: 1}, []byte(helloPayload))
	if s.metrics.DroppedFilter.Load() != 1 {
		t.Fatalf("filter drops = %d", s.metrics.DroppedFilter.Load())
	}
	if len(h.snapshot()) != 0 {
		t.Fatal("classified drop stored")
	}
}

type errHandler struct{}

func (errHandler) Insert(context.Context, model.Message) error { return errors.New("store_full") }

func TestFirstMatchWinsOverBroaderCIDR(t *testing.T) {
	h := newFakeHandler()
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
	a := mustAdmission(t, model.Admission{AllowClientCIDRs: []string{"10.99.42.0/24"}})
	s := testPipeline(h, Config{Admission: a, Classifier: c})
	if s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("10.99.42.1"), port: 1}, []byte(helloPayload)) {
		t.Fatal("filter drop must not close")
	}
	if s.metrics.DroppedFilter.Load() != 1 {
		t.Fatalf("narrower drop missing: %d", s.metrics.DroppedFilter.Load())
	}
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("10.99.42.2"), port: 1}, []byte(helloPayload))
	m := waitMsg(t, h)
	if m.RemoteIP.String() != "10.99.42.2" {
		t.Fatalf("captured %s", m.RemoteIP)
	}
	if len(h.snapshot()) != 1 {
		t.Fatalf("stored %d, want 1 (first-match drop, later capture)", len(h.snapshot()))
	}
}

func TestTagActionSetsMessageTags(t *testing.T) {
	h := newFakeHandler()
	c := mustClassifier(t, []model.Filter{{
		Name:    "tag-all",
		Enabled: boolPtr(true),
		Action:  model.FilterAction{Mode: ActionTag, Tag: "lab"},
	}})
	s := testPipeline(h, Config{Classifier: c})
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("127.0.0.1"), port: 1}, []byte(helloPayload))
	m := waitMsg(t, h)
	if len(m.Tags) != 1 || m.Tags[0] != "lab" {
		t.Fatalf("tags = %q", m.Tags)
	}
}

func TestNoFilterAllowListHitCaptures(t *testing.T) {
	h := newFakeHandler()
	a := mustAdmission(t, model.Admission{AllowClientCIDRs: []string{"127.0.0.0/8", "::1/128"}})
	c := mustClassifier(t, nil)
	s := testPipeline(h, Config{Admission: a, Classifier: c})
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("127.0.0.1"), port: 1}, []byte(helloPayload))
	m := waitMsg(t, h)
	if len(m.Tags) != 0 {
		t.Fatalf("unmatched capture tags = %q", m.Tags)
	}
	if s.metrics.Stored.Load() != 1 {
		t.Fatalf("stored = %d", s.metrics.Stored.Load())
	}
}

func TestAdmissionCIDRMissNotStored(t *testing.T) {
	h := newFakeHandler()
	a := mustAdmission(t, model.Admission{AllowClientCIDRs: []string{"10.99.42.0/24"}})
	s := testPipeline(h, Config{Admission: a})
	if !s.ingest(context.Background(), TransportTCP, remoteAddr{ip: netip.MustParseAddr("192.0.2.1"), port: 1}, []byte(helloPayload)) {
		t.Fatal("CIDR miss must close TCP")
	}
	if s.metrics.DroppedAdmission.Load() != 1 {
		t.Fatalf("admission drops = %d", s.metrics.DroppedAdmission.Load())
	}
	if len(h.snapshot()) != 0 {
		t.Fatal("CIDR miss stored a message")
	}
}

func TestHandlerErrorIsStoreFullDrop(t *testing.T) {
	s := testPipeline(errHandler{}, Config{})
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("127.0.0.1"), port: 1}, []byte(helloPayload))
	if s.metrics.DroppedStore.Load() != 1 {
		t.Fatalf("store drops = %d", s.metrics.DroppedStore.Load())
	}
	if s.metrics.Stored.Load() != 0 {
		t.Fatal("stored on handler error")
	}
}
