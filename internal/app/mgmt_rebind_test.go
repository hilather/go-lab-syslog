package app

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

// Reset must not replace the management listener cmd/labsyslog is already
// serving. An effective address change is validation_failed and the old
// socket keeps answering HTTP.
func TestResetManagementRebindKeepsHTTPServing(t *testing.T) {
	addr1 := reserveTCP(t)
	addr2 := reserveTCP(t)
	lab := newMgmtLab(t, compiler.Options{}, "127.0.0.1:0", "127.0.0.1:0", addr1)
	ctx := testutil.Context(t)
	if err := lab.svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	ln := lab.svc.ManagementListener()
	if ln == nil {
		t.Fatal("management listener was not bound")
	}
	serveHealth(t, ln)
	getLive(t, addr1)

	if _, err := lab.svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("keep")}); err != nil {
		t.Fatal(err)
	}
	rev := lab.svc.State(ctx).Revision
	messages := lab.svc.Messages().Stats().Messages

	lab.write("127.0.0.1:0", "127.0.0.1:0", addr2)
	err := lab.svc.Reset(ctx, "operator", "move management")
	if err == nil {
		t.Fatalf("reset moved management to %s and returned nil", lab.svc.ManagementAddr())
	}
	assertRestartRefusal(t, err)
	getLive(t, addr1)
	assertNotListening(t, addr2)
	if got := lab.svc.Messages().Stats().Messages; got != messages {
		t.Fatalf("messages %d -> %d", messages, got)
	}
	if got := lab.svc.State(ctx).Revision; got != rev {
		t.Fatalf("revision %s -> %s", rev, got)
	}
}

func TestResetRefusesManagementUnbind(t *testing.T) {
	addr := reserveTCP(t)
	lab := newMgmtLab(t, compiler.Options{}, "127.0.0.1:0", "127.0.0.1:0", addr)
	ctx := testutil.Context(t)
	if err := lab.svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	ln := lab.svc.ManagementListener()
	if ln == nil {
		t.Fatal("management listener was not bound")
	}
	serveHealth(t, ln)
	getLive(t, addr)
	if _, err := lab.svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("keep")}); err != nil {
		t.Fatal(err)
	}
	rev := lab.svc.State(ctx).Revision
	messages := lab.svc.Messages().Stats().Messages

	lab.write("127.0.0.1:0", "127.0.0.1:0", "")
	err := lab.svc.Reset(ctx, "operator", "clear management")
	if err == nil {
		t.Fatal("reset cleared the management address and returned nil")
	}
	assertRestartRefusal(t, err)
	getLive(t, addr)
	if got := lab.svc.Messages().Stats().Messages; got != messages {
		t.Fatalf("messages %d -> %d", messages, got)
	}
	if got := lab.svc.State(ctx).Revision; got != rev {
		t.Fatalf("revision %s -> %s", rev, got)
	}
}

func TestResetRefusesManagementEnable(t *testing.T) {
	addr := reserveTCP(t)
	lab := newMgmtLab(t, compiler.Options{}, "127.0.0.1:0", "127.0.0.1:0", "")
	ctx := testutil.Context(t)
	if err := lab.svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if lab.svc.ManagementListener() != nil {
		t.Fatal("management listener bound from an empty address")
	}
	if _, err := lab.svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("keep")}); err != nil {
		t.Fatal(err)
	}
	rev := lab.svc.State(ctx).Revision
	messages := lab.svc.Messages().Stats().Messages

	lab.write("127.0.0.1:0", "127.0.0.1:0", addr)
	err := lab.svc.Reset(ctx, "operator", "enable management")
	if err == nil {
		t.Fatal("reset enabled management and returned nil")
	}
	assertRestartRefusal(t, err)
	assertNotListening(t, addr)
	if got := lab.svc.Messages().Stats().Messages; got != messages {
		t.Fatalf("messages %d -> %d", messages, got)
	}
	if got := lab.svc.State(ctx).Revision; got != rev {
		t.Fatalf("revision %s -> %s", rev, got)
	}
	if lab.svc.ManagementListener() != nil {
		t.Fatal("management listener replaced")
	}
}

// --management-listen=off wins over a YAML address that changes. Reset
// still wipes and rebinds UDP and TCP.
func TestResetManagementFlagOffIgnoresYAMLAddress(t *testing.T) {
	udp2 := reserveUDP(t)
	tcp2 := reserveTCP(t)
	lab := newMgmtLab(t, compiler.Options{ManagementListen: "off"}, "127.0.0.1:0", "127.0.0.1:0", reserveTCP(t))
	ctx := testutil.Context(t)
	if err := lab.svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if lab.svc.ManagementListener() != nil {
		t.Fatal("management bound while --management-listen=off")
	}
	if _, err := lab.svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("wipe-me")}); err != nil {
		t.Fatal(err)
	}
	oldUDP := lab.svc.UDPAddr().String()
	oldTCP := lab.svc.TCPAddr().String()

	lab.write(udp2, tcp2, reserveTCP(t))
	if err := lab.svc.Reset(ctx, "operator", "flag off"); err != nil {
		t.Fatal(err)
	}
	if lab.svc.Messages().Stats().Messages != 0 {
		t.Fatal("reset did not wipe")
	}
	if got := lab.svc.UDPAddr().String(); got != udp2 || got == oldUDP {
		t.Fatalf("udp %s -> %s, want %s", oldUDP, got, udp2)
	}
	if got := lab.svc.TCPAddr().String(); got != tcp2 || got == oldTCP {
		t.Fatalf("tcp %s -> %s, want %s", oldTCP, got, tcp2)
	}
	if lab.svc.ManagementListener() != nil {
		t.Fatal("management bound after reset with flag off")
	}
	if got := lab.svc.Snapshot().Document.Spec.Listeners.Management.Address; got != "" {
		t.Fatalf("effective management address %q", got)
	}
}

// A stable non-off --management-listen wins over a different YAML address.
func TestResetUnchangedManagementFlagIgnoresYAMLAddress(t *testing.T) {
	flagAddr := reserveTCP(t)
	lab := newMgmtLab(t, compiler.Options{ManagementListen: flagAddr}, "127.0.0.1:0", "127.0.0.1:0", reserveTCP(t))
	ctx := testutil.Context(t)
	if err := lab.svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	ln := lab.svc.ManagementListener()
	if ln == nil {
		t.Fatal("management listener was not bound")
	}
	serveHealth(t, ln)
	getLive(t, flagAddr)
	if _, err := lab.svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("wipe-me")}); err != nil {
		t.Fatal(err)
	}

	yamlAddr := reserveTCP(t)
	lab.write("127.0.0.1:0", "127.0.0.1:0", yamlAddr)
	if err := lab.svc.Reset(ctx, "operator", "yaml only"); err != nil {
		t.Fatal(err)
	}
	if lab.svc.Messages().Stats().Messages != 0 {
		t.Fatal("reset did not wipe")
	}
	getLive(t, flagAddr)
	assertNotListening(t, yamlAddr)
	if got := lab.svc.Snapshot().Document.Spec.Listeners.Management.Address; got != flagAddr {
		t.Fatalf("effective management address %q, want flag %s", got, flagAddr)
	}
}

type mgmtLab struct {
	t    *testing.T
	svc  *Service
	path string
	tok  string
}

func newMgmtLab(t *testing.T, opts compiler.Options, udp, tcp, mgmt string) *mgmtLab {
	t.Helper()
	dir := t.TempDir()
	tok := dir + "/token"
	if err := os.WriteFile(tok, []byte(strings.Repeat("t", auth.MinTokenBytes)), 0o600); err != nil {
		t.Fatal(err)
	}
	path := dir + "/config.yaml"
	lab := &mgmtLab{t: t, path: path, tok: tok}
	lab.write(udp, tcp, mgmt)
	opts.ConfigDir = dir
	svc, err := New(Config{BootstrapPath: path, Compiler: opts})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	lab.svc = svc
	return lab
}

func (l *mgmtLab) write(udp, tcp, mgmt string) {
	l.t.Helper()
	body := fmt.Sprintf(`apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec:
  listeners:
    udp:
      enabled: true
      address: %q
    tcp:
      enabled: true
      address: %q
    management:
      address: %q
      restPath: /v1
      mcpPath: /mcp
  auth:
    tokens:
      - id: operator
        role: administrator
        secretFile: %q
  admission:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
`, udp, tcp, mgmt, l.tok)
	if err := os.WriteFile(l.path, []byte(body), 0o644); err != nil {
		l.t.Fatal(err)
	}
}

func reserveTCP(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func reserveUDP(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	if err := pc.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func serveHealth(t *testing.T, ln net.Listener) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
}

func getLive(t *testing.T, addr string) {
	t.Helper()
	client := &http.Client{Timeout: 400 * time.Millisecond}
	resp, err := client.Get("http://" + addr + "/v1/health/live")
	if err != nil {
		t.Fatalf("GET %s: %v", addr, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status %d", addr, resp.StatusCode)
	}
}

func assertNotListening(t *testing.T, addr string) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err == nil {
		c.Close()
		t.Fatalf("%s accepted a connection", addr)
	}
}

func assertRestartRefusal(t *testing.T, err error) {
	t.Helper()
	if !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("reset err %v, want validation_failed", err)
	}
	de, _ := domainerr.As(err)
	if de == nil || !strings.Contains(de.Detail, "process restart") {
		t.Fatalf("detail %v, want process restart", err)
	}
}
