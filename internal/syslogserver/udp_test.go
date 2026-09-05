package syslogserver

import (
	"bytes"
	"context"
	"go/parser"
	"go/token"
	"net"
	"net/netip"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

const helloPayload = "<14>hello\n"

type fakeHandler struct {
	mu  sync.Mutex
	got []model.Message
	ch  chan model.Message
}

func newFakeHandler() *fakeHandler {
	return &fakeHandler{ch: make(chan model.Message, 32)}
}

func (f *fakeHandler) Insert(_ context.Context, msg model.Message) error {
	f.mu.Lock()
	f.got = append(f.got, msg)
	f.mu.Unlock()
	select {
	case f.ch <- msg:
	default:
	}
	return nil
}

func (f *fakeHandler) snapshot() []model.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]model.Message, len(f.got))
	copy(out, f.got)
	return out
}

func startUDP(t *testing.T, cfg Config) (*Server, *fakeHandler) {
	t.Helper()
	h := newFakeHandler()
	cfg.Handler = h
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:0"
	}
	srv, err := ListenUDP(testutil.Context(t), cfg)
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	testutil.Cleanup(t, func() { _ = srv.Close() })
	return srv, h
}

func waitMsg(t *testing.T, h *fakeHandler) model.Message {
	t.Helper()
	select {
	case m := <-h.ch:
		return m
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Handler.Insert")
		return model.Message{}
	}
}

func waitMetric(t *testing.T, get func() uint64, want uint64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if get() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("metric = %d, want >= %d", get(), want)
}

func sendUDP(t *testing.T, network, address string, payload []byte) {
	t.Helper()
	c, err := net.Dial(network, address)
	if err != nil {
		t.Fatalf("dial %s %s: %v", network, address, err)
	}
	defer c.Close()
	if _, err := c.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestDualStackFakeHandler(t *testing.T) {
	h := newFakeHandler()
	var srv *Server
	var err error
	ctx := testutil.Context(t)
	for _, addr := range []string{"[::]:0", ":0"} {
		srv, err = ListenUDP(ctx, Config{Addr: addr, Handler: h, Parse: syslogwire.DefaultOptions()})
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("ListenUDP dual-stack: %v", err)
	}
	testutil.Cleanup(t, func() { _ = srv.Close() })

	_, port, err := net.SplitHostPort(srv.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}

	type probe struct {
		name    string
		network string
		address string
		wantIP  netip.Addr
	}
	probes := []probe{
		{"ipv4", "udp4", net.JoinHostPort("127.0.0.1", port), netip.MustParseAddr("127.0.0.1")},
		{"ipv6", "udp6", net.JoinHostPort("::1", port), netip.MustParseAddr("::1")},
	}
	var got int
	for _, p := range probes {
		c, err := net.Dial(p.network, p.address)
		if err != nil {
			t.Logf("%s not available: %v", p.name, err)
			continue
		}
		if _, err := c.Write([]byte(helloPayload)); err != nil {
			_ = c.Close()
			t.Logf("%s write: %v", p.name, err)
			continue
		}
		_ = c.Close()
		m := waitMsg(t, h)
		if m.Truncated {
			t.Fatal("truncated must stay false")
		}
		if m.Transport != TransportUDP {
			t.Fatalf("transport = %q", m.Transport)
		}
		if !bytes.Equal(m.Raw, []byte(helloPayload)) {
			t.Fatalf("raw = %q", m.Raw)
		}
		if m.RemoteIP.Is4In6() {
			t.Fatalf("IPv4-mapped address not unmapped: %s", m.RemoteIP)
		}
		if m.RemoteIP != p.wantIP {
			t.Fatalf("remote IP = %s, want %s", m.RemoteIP, p.wantIP)
		}
		got++
		t.Logf("%s remote %s", p.name, m.RemoteIP)
	}
	if got != 2 {
		t.Fatalf("dual-stack requires 127.0.0.1 and ::1, got %d families", got)
	}
}

func TestOversizeDatagramDropped(t *testing.T) {
	srv, h := startUDP(t, Config{
		Addr:                "127.0.0.1:0",
		UDPMaxDatagramBytes: 16,
		MaxMessageBytes:     16,
	})
	sendUDP(t, "udp4", srv.LocalAddr().String(), bytes.Repeat([]byte("x"), 32))
	waitMetric(t, srv.Metrics().DroppedOversize.Load, 1)
	waitMetric(t, srv.Metrics().UDPOversize.Load, 1)
	if n := len(h.snapshot()); n != 0 {
		t.Fatalf("stored %d messages, want 0", n)
	}
	if srv.Metrics().Stored.Load() != 0 {
		t.Fatal("stored counter incremented on oversize")
	}
}

func TestMaxMessageBytesOversizeDropped(t *testing.T) {
	srv, h := startUDP(t, Config{
		Addr:                "127.0.0.1:0",
		UDPMaxDatagramBytes: 64,
		MaxMessageBytes:     8,
	})
	sendUDP(t, "udp4", srv.LocalAddr().String(), []byte("<14>hello-too-long\n"))
	waitMetric(t, srv.Metrics().DroppedOversize.Load, 1)
	if n := len(h.snapshot()); n != 0 {
		t.Fatalf("stored %d messages, want 0", n)
	}
}

func TestEmptyDatagramDropped(t *testing.T) {
	srv, h := startUDP(t, Config{Addr: "127.0.0.1:0"})
	sendUDP(t, "udp4", srv.LocalAddr().String(), nil)
	waitMetric(t, srv.Metrics().DroppedEmpty.Load, 1)
	if n := len(h.snapshot()); n != 0 {
		t.Fatalf("stored %d messages, want 0", n)
	}
}

func TestPrintfHelloReachesFakeHandler(t *testing.T) {
	srv, h := startUDP(t, Config{Addr: "127.0.0.1:0"})
	host, port, err := net.SplitHostPort(srv.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	sendPrintfHello(t, host, port)
	m := waitMsg(t, h)
	if !bytes.Equal(m.Raw, []byte(helloPayload)) && !bytes.Equal(m.Raw, []byte("<14>hello")) {
		t.Fatalf("raw = %q, want %q", m.Raw, helloPayload)
	}
	if m.Truncated {
		t.Fatal("truncated must stay false")
	}
	if m.Parsed.PRI != 14 {
		t.Fatalf("PRI = %d, want 14", m.Parsed.PRI)
	}
}

func sendPrintfHello(t *testing.T, host, port string) {
	t.Helper()
	if nc, err := exec.LookPath("nc"); err == nil {
		cmd := exec.Command(nc, "-u", "-w", "1", host, port)
		cmd.Stdin = strings.NewReader(helloPayload)
		if out, err := cmd.CombinedOutput(); err == nil {
			return
		} else {
			t.Logf("nc -u failed (%v): %s", err, out)
		}
	}
	sendUDP(t, "udp4", net.JoinHostPort(host, port), []byte(helloPayload))
}

func TestUDPCallSiteDoesNotImportSyslogwire(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "udp.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(path, "syslogwire") {
			t.Fatalf("udp.go imports %s; UDP call site must not parse before ingest", path)
		}
		if path == "net/http" {
			t.Fatal("udp.go imports net/http")
		}
	}
}

func TestUnmapIPv4Mapped(t *testing.T) {
	mapped := netip.MustParseAddr("::ffff:127.0.0.1")
	got := mapped.Unmap()
	if got.String() != "127.0.0.1" || got.Is4In6() {
		t.Fatalf("unmap %s = %s", mapped, got)
	}
	h := newFakeHandler()
	s := &Server{
		cfg:     applyDefaults(Config{Handler: h}),
		metrics: &Metrics{},
		ctx:     context.Background(),
	}
	s.ingest(context.Background(), TransportUDP, remoteAddr{ip: mapped, port: 514}, []byte(helloPayload))
	m := waitMsg(t, h)
	if m.RemoteIP.String() != "127.0.0.1" {
		t.Fatalf("ingest remote = %s, want 127.0.0.1", m.RemoteIP)
	}
}
