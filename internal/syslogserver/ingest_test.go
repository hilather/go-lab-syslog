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
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("127.0.0.1"), port: 1}, []byte(helloPayload))
	if s.metrics.DroppedBehavior.Load() != 1 {
		t.Fatalf("behavior drops = %d", s.metrics.DroppedBehavior.Load())
	}
	if len(h.snapshot()) != 0 {
		t.Fatal("behavior drop stored a message")
	}
}

type denyAll struct{}

func (denyAll) Allow(netip.Addr) (bool, string) { return false, "cidr" }

func TestAdmissionDenySkipsHandler(t *testing.T) {
	h := newFakeHandler()
	s := testPipeline(h, Config{Admission: denyAll{}})
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: netip.MustParseAddr("10.0.0.1"), port: 1}, []byte(helloPayload))
	if s.metrics.DroppedAdmission.Load() != 1 {
		t.Fatalf("admission drops = %d", s.metrics.DroppedAdmission.Load())
	}
	if s.metrics.Received.Load() != 0 {
		t.Fatal("received counts pre-admission")
	}
	if len(h.snapshot()) != 0 {
		t.Fatal("denied message stored")
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
