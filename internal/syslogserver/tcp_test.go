package syslogserver

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/syslogframing"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

const docsOctetMSG = "<165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 - - hello"

func startTCP(t *testing.T, cfg Config) (*Server, *fakeHandler) {
	t.Helper()
	h := newFakeHandler()
	cfg.Handler = h
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:0"
	}
	srv, err := ListenTCP(testutil.Context(t), cfg)
	if err != nil {
		t.Fatalf("ListenTCP: %v", err)
	}
	testutil.Cleanup(t, func() { _ = srv.Close() })
	return srv, h
}

func sendTCP(t *testing.T, address string, payload []byte) {
	t.Helper()
	c, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("dial tcp %s: %v", address, err)
	}
	defer c.Close()
	if _, err := c.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func octetFrame(msg string) []byte {
	return []byte(strconv.Itoa(len(msg)) + " " + msg)
}

func TestTCPOctetTwoBackToBack(t *testing.T) {
	srv, h := startTCP(t, Config{Addr: "127.0.0.1:0", Framing: syslogframing.OctetCounting})
	payload := append(octetFrame(docsOctetMSG), octetFrame(docsOctetMSG)...)
	sendTCP(t, srv.LocalAddr().String(), payload)
	m1 := waitMsg(t, h)
	m2 := waitMsg(t, h)
	if m1.Transport != TransportTCP || m2.Transport != TransportTCP {
		t.Fatalf("transport %q %q", m1.Transport, m2.Transport)
	}
	if m1.Truncated || m2.Truncated {
		t.Fatal("truncated")
	}
	if !bytes.Equal(m1.Raw, []byte(docsOctetMSG)) || !bytes.Equal(m2.Raw, []byte(docsOctetMSG)) {
		t.Fatalf("raw = %q %q", m1.Raw, m2.Raw)
	}
}

func TestTCPNLTwoMessages(t *testing.T) {
	srv, h := startTCP(t, Config{Addr: "127.0.0.1:0", Framing: syslogframing.NonTransparent})
	sendTCP(t, srv.LocalAddr().String(), []byte("<14>hello\n<15>world\n"))
	m1 := waitMsg(t, h)
	m2 := waitMsg(t, h)
	if !bytes.Equal(m1.Raw, []byte("<14>hello")) || !bytes.Equal(m2.Raw, []byte("<15>world")) {
		t.Fatalf("raw = %q %q", m1.Raw, m2.Raw)
	}
}

func TestTCPAutoDigitSP(t *testing.T) {
	srv, h := startTCP(t, Config{Addr: "127.0.0.1:0", Framing: syslogframing.Auto})
	payload := append(octetFrame("<14>hello"), octetFrame("<15>world")...)
	sendTCP(t, srv.LocalAddr().String(), payload)
	m1 := waitMsg(t, h)
	m2 := waitMsg(t, h)
	if !bytes.Equal(m1.Raw, []byte("<14>hello")) || !bytes.Equal(m2.Raw, []byte("<15>world")) {
		t.Fatalf("raw = %q %q", m1.Raw, m2.Raw)
	}
}

func TestTCPAutoPRINotFlipped(t *testing.T) {
	srv, h := startTCP(t, Config{Addr: "127.0.0.1:0", Framing: syslogframing.Auto})
	sendTCP(t, srv.LocalAddr().String(), []byte("<14>hello\n9 <14>hello"))
	m := waitMsg(t, h)
	if !bytes.Equal(m.Raw, []byte("<14>hello")) {
		t.Fatalf("raw = %q", m.Raw)
	}
	waitMetric(t, srv.Metrics().TCPFramingErrors.Load, 1)
	if n := len(h.snapshot()); n != 1 {
		t.Fatalf("stored %d, want 1 (PRI must not flip auto to octet-counting)", n)
	}
}

func TestTCPOctetOversizeClosesNoStore(t *testing.T) {
	srv, h := startTCP(t, Config{
		Addr:            "127.0.0.1:0",
		Framing:         syslogframing.OctetCounting,
		MaxMessageBytes: 8,
	})
	c, err := net.Dial("tcp", srv.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("20 <14>this-is-too-long")); err != nil {
		t.Fatal(err)
	}
	waitMetric(t, srv.Metrics().TCPFramingErrors.Load, 1)
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	if _, err := c.Read(buf); err == nil {
		t.Fatal("over-length octet frame must close the session")
	}
	if n := len(h.snapshot()); n != 0 {
		t.Fatalf("stored %d partial messages", n)
	}
	if srv.Metrics().Stored.Load() != 0 {
		t.Fatal("stored counter incremented on octet oversize")
	}
	if srv.Metrics().UDPOversize.Load() != 0 {
		t.Fatal("TCP octet oversize counted as UDP oversize")
	}
}

func TestTCPNonTransparentOversizeDropsFrame(t *testing.T) {
	srv, h := startTCP(t, Config{
		Addr:            "127.0.0.1:0",
		Framing:         syslogframing.NonTransparent,
		MaxMessageBytes: 8,
	})
	sendTCP(t, srv.LocalAddr().String(), []byte("123456789\n<14>ok\n"))
	m := waitMsg(t, h)
	if !bytes.Equal(m.Raw, []byte("<14>ok")) {
		t.Fatalf("raw = %q", m.Raw)
	}
	waitMetric(t, srv.Metrics().DroppedOversize.Load, 1)
	if srv.Metrics().UDPOversize.Load() != 0 {
		t.Fatal("TCP non-transparent oversize counted as UDP oversize")
	}
	if srv.Metrics().TCPFramingErrors.Load() != 0 {
		t.Fatal("non-transparent oversize is not a framing close")
	}
}

func TestTCPIdleTimeout(t *testing.T) {
	srv, h := startTCP(t, Config{
		Addr:           "127.0.0.1:0",
		TCPIdleTimeout: 150 * time.Millisecond,
	})
	c, err := net.Dial("tcp", srv.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	if _, err := c.Read(buf); err == nil {
		t.Fatal("idle session must close")
	}
	if n := len(h.snapshot()); n != 0 {
		t.Fatalf("idle stored %d", n)
	}
}

func TestTCPNULAndCRLF(t *testing.T) {
	srv, h := startTCP(t, Config{Addr: "127.0.0.1:0", Framing: syslogframing.NonTransparent})
	sendTCP(t, srv.LocalAddr().String(), []byte("<14>hello\x00<15>world\r\n"))
	m1 := waitMsg(t, h)
	m2 := waitMsg(t, h)
	if !bytes.Equal(m1.Raw, []byte("<14>hello")) || !bytes.Equal(m2.Raw, []byte("<15>world")) {
		t.Fatalf("raw = %q %q", m1.Raw, m2.Raw)
	}
}

func TestTCPOctetFromPythonOrNC(t *testing.T) {
	srv, h := startTCP(t, Config{Addr: "127.0.0.1:0", Framing: syslogframing.OctetCounting})
	host, port, err := net.SplitHostPort(srv.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	payload := append(octetFrame(docsOctetMSG), octetFrame(docsOctetMSG)...)
	sendOctetPythonOrNC(t, host, port, payload)
	m1 := waitMsg(t, h)
	m2 := waitMsg(t, h)
	if !bytes.Equal(m1.Raw, []byte(docsOctetMSG)) || !bytes.Equal(m2.Raw, []byte(docsOctetMSG)) {
		t.Fatalf("raw = %q %q", m1.Raw, m2.Raw)
	}
	if m1.Parsed.PRI != 165 || m2.Parsed.PRI != 165 {
		t.Fatalf("PRI = %d %d", m1.Parsed.PRI, m2.Parsed.PRI)
	}
}

func sendOctetPythonOrNC(t *testing.T, host, port string, payload []byte) {
	t.Helper()
	if py, err := exec.LookPath("python3"); err == nil {
		script := fmt.Sprintf(
			"import socket,sys; s=socket.create_connection((sys.argv[1], int(sys.argv[2]))); s.sendall(sys.stdin.buffer.read()); s.close()",
		)
		cmd := exec.Command(py, "-c", script, host, port)
		cmd.Stdin = bytes.NewReader(payload)
		if out, err := cmd.CombinedOutput(); err == nil {
			return
		} else {
			t.Logf("python3 failed (%v): %s", err, out)
		}
	}
	if nc, err := exec.LookPath("nc"); err == nil {
		cmd := exec.Command(nc, "-w", "1", host, port)
		cmd.Stdin = bytes.NewReader(payload)
		if out, err := cmd.CombinedOutput(); err == nil {
			return
		} else {
			t.Logf("nc failed (%v): %s", err, out)
		}
	}
	sendTCP(t, net.JoinHostPort(host, port), payload)
}

func TestTCPMaxConnsRejects(t *testing.T) {
	srv, h := startTCP(t, Config{Addr: "127.0.0.1:0", MaxTCPConns: 1})
	c1, err := net.Dial("tcp", srv.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()
	waitMetric(t, func() uint64 { return uint64(srv.Metrics().TCPConns.Load()) }, 1)
	c2, err := net.Dial("tcp", srv.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := c2.Read(make([]byte, 1)); err == nil {
		t.Fatal("excess connection should be closed")
	}
	if _, err := c1.Write([]byte("<14>held\n")); err != nil {
		t.Fatal(err)
	}
	m := waitMsg(t, h)
	if !bytes.Equal(m.Raw, []byte("<14>held")) {
		t.Fatalf("raw = %q", m.Raw)
	}
	if n := len(h.snapshot()); n != 1 {
		t.Fatalf("stored %d", n)
	}
}

func TestTCPCallSiteDoesNotImportSyslogwire(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "tcp.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(path, "syslogwire") {
			t.Fatalf("tcp.go imports %s; TCP call site must not parse before ingest", path)
		}
		if path == "net/http" {
			t.Fatal("tcp.go imports net/http")
		}
	}
}

func TestTCPInvalidFramingRejected(t *testing.T) {
	srv, err := ListenTCP(testutil.Context(t), Config{Addr: "127.0.0.1:0", Framing: "newline"})
	if err == nil {
		_ = srv.Close()
		t.Fatal("abbreviation newline must not be accepted")
	}
	if !strings.Contains(err.Error(), "octet-counting") || !strings.Contains(err.Error(), "non-transparent") {
		t.Fatalf("error %v must use frozen framing names", err)
	}
}

func TestTCPHalfCloseStoresCompleteFrame(t *testing.T) {
	srv, h := startTCP(t, Config{Addr: "127.0.0.1:0", Framing: syslogframing.NonTransparent})
	c, err := net.Dial("tcp", srv.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("<14>hello\n")); err != nil {
		t.Fatal(err)
	}
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	} else {
		_ = c.Close()
	}
	m := waitMsg(t, h)
	if !bytes.Equal(m.Raw, []byte("<14>hello")) {
		t.Fatalf("raw = %q", m.Raw)
	}
}
