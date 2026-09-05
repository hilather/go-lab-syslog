package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/store"
	"github.com/hilather/go-lab-syslog/internal/syslogserver"
	"github.com/hilather/go-lab-syslog/internal/syslogtest"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

type lockedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.String()
}

func TestServeMissingTokenFailClosedWhenManagementBound(t *testing.T) {
	cfg := filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml")
	var stdout, stderr bytes.Buffer
	code := cmdServe(context.Background(), []string{
		"--config", cfg,
		"--syslog-udp-listen", "127.0.0.1:0",
		"--syslog-tcp-listen", "127.0.0.1:0",
		"--management-listen", "127.0.0.1:0",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("missing token must fail closed when management is bound")
	}
	if !strings.Contains(stderr.String(), "secretFile") && !strings.Contains(stderr.String(), "token") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestServeManagementBoundWithToken(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, bytes.Repeat([]byte("t"), 32), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.yaml")
	body := []byte(`
apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec:
  auth:
    tokens:
      - id: operator
        secretFile: ` + tok + `
`)
	if err := os.WriteFile(cfg, body, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr lockedBuffer
	done := make(chan int, 1)
	go func() {
		done <- cmdServe(ctx, []string{
			"--config", cfg,
			"--syslog-udp-listen", "127.0.0.1:0",
			"--syslog-tcp-listen", "127.0.0.1:0",
			"--management-listen", "127.0.0.1:0",
		}, &stdout, &stderr)
	}()
	_ = waitListenAddr(t, &stderr)
	if !strings.Contains(stderr.String(), "management listen 127.0.0.1:") {
		t.Fatalf("stderr %q missing management listen", stderr.String())
	}
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("serve exit %d stderr=%q", code, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit after cancel")
	}
}

func TestServeRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "serve"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--config") {
		t.Fatalf("stderr %q missing --config", stderr.String())
	}
}

func TestServeManagementOffAcceptsUDP(t *testing.T) {
	cfg := filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr lockedBuffer
	done := make(chan int, 1)
	go func() {
		done <- cmdServe(ctx, []string{
			"--config", cfg,
			"--syslog-udp-listen", "127.0.0.1:0",
			"--syslog-tcp-listen", "127.0.0.1:0",
			"--management-listen", "off",
		}, &stdout, &stderr)
	}()

	addr := waitListenAddr(t, &stderr)
	if !strings.Contains(stderr.String(), "management listen off") {
		t.Fatalf("stderr %q missing management listen off", stderr.String())
	}

	c, err := net.Dial("udp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write([]byte("<14>hello\n")); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()

	select {
	case code := <-done:
		t.Fatalf("serve exited %d before cancel; stderr=%q", code, stderr.String())
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("serve exit %d stderr=%q", code, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit after cancel")
	}
}

func TestServeManagementOffAcceptsTCP(t *testing.T) {
	cfg := filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr lockedBuffer
	done := make(chan int, 1)
	go func() {
		done <- cmdServe(ctx, []string{
			"--config", cfg,
			"--syslog-udp-listen", "127.0.0.1:0",
			"--syslog-tcp-listen", "127.0.0.1:0",
			"--management-listen", "off",
		}, &stdout, &stderr)
	}()

	addr := waitListenPrefix(t, &stderr, "syslog tcp listen ")
	if !strings.Contains(stderr.String(), "management listen off") {
		t.Fatalf("stderr %q missing management listen off", stderr.String())
	}

	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write([]byte("9 <14>hello")); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()

	select {
	case code := <-done:
		t.Fatalf("serve exited %d before cancel; stderr=%q", code, stderr.String())
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("serve exit %d stderr=%q", code, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit after cancel")
	}
}

func waitListenAddr(t *testing.T, stderr *lockedBuffer) string {
	t.Helper()
	return waitListenPrefix(t, stderr, "syslog udp listen ")
}

const (
	rfc3164Probe = "<34>Sep  4 20:52:35 sut-1 sshd[1234]: Failed password"
	rfc5424Probe = `<165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 ID47 [sshd@0 user="alice"] Failed password`
)

func TestServeStoreHandlerUDP3164AndTCP5424(t *testing.T) {
	svc := startServeService(t, "")
	udpAddr := svc.UDPAddr()
	tcpAddr := svc.TCPAddr()
	if udpAddr == nil || tcpAddr == nil {
		t.Fatal("udp/tcp not bound")
	}

	c, err := net.Dial("udp4", udpAddr.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write([]byte(rfc3164Probe)); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	udpMsg := syslogtest.Wait(t, svc.Messages(), store.ListFilter{Transport: store.TransportUDP}, 0)
	if udpMsg.Message.Parsed.Version != 0 || udpMsg.Message.Parsed.PRI != 34 {
		t.Fatalf("udp parsed=%+v", udpMsg.Message.Parsed)
	}

	tc, err := net.Dial("tcp", tcpAddr.String())
	if err != nil {
		t.Fatal(err)
	}
	frame := strconv.Itoa(len(rfc5424Probe)) + " " + rfc5424Probe
	if _, err := tc.Write([]byte(frame)); err != nil {
		t.Fatal(err)
	}
	_ = tc.Close()
	tcpMsg := syslogtest.Wait(t, svc.Messages(), store.ListFilter{Transport: store.TransportTCP}, 0)
	if tcpMsg.Message.Parsed.Version != 1 || tcpMsg.Message.Parsed.PRI != 165 {
		t.Fatalf("tcp parsed=%+v", tcpMsg.Message.Parsed)
	}

	listed, err := svc.Messages().List(store.ListFilter{}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 2 {
		t.Fatalf("stored %d, want 2", len(listed.Items))
	}
}

func TestServeBehaviorFromSpecDropSilent(t *testing.T) {
	svc := startServeService(t, `
  syslog:
    behavior:
      mode: drop-silent
`)
	if svc.Snapshot().Document.Spec.Syslog.Behavior.Mode != syslogserver.BehaviorDropSilent {
		t.Fatalf("behavior = %q", svc.Snapshot().Document.Spec.Syslog.Behavior.Mode)
	}
	addr := svc.UDPAddr()
	if addr == nil {
		t.Fatal("udp not bound")
	}
	c, err := net.Dial("udp4", addr.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write([]byte(rfc3164Probe)); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if svc.Metrics().DroppedBehavior.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if svc.Metrics().DroppedBehavior.Load() < 1 {
		t.Fatal("drop-silent did not discard")
	}
	if n := svc.Messages().Stats().Messages; n != 0 {
		t.Fatalf("stored %d after drop-silent", n)
	}
}

func startServeService(t *testing.T, extraSpec string) *app.Service {
	t.Helper()
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, bytes.Repeat([]byte("t"), 32), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec:
  listeners:
    udp:
      enabled: true
      address: "127.0.0.1:0"
    tcp:
      enabled: true
      address: "127.0.0.1:0"
  auth:
    tokens:
      - id: operator
        secretFile: ` + tok + `
  admission:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
` + extraSpec
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.New(app.Config{
		BootstrapPath: path,
		Compiler: compiler.Options{
			ConfigDir:        dir,
			ManagementListen: "off",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Start(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
	testutil.Cleanup(t, func() { _ = svc.Close() })
	return svc
}

func waitListenPrefix(t *testing.T, stderr *lockedBuffer, prefix string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s := stderr.String()
		for _, line := range strings.Split(s, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, prefix) {
				return strings.TrimPrefix(line, prefix)
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q; stderr=%q", prefix, stderr.String())
	return ""
}
