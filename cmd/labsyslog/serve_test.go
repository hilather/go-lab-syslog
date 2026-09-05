package main

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

func waitListenAddr(t *testing.T, stderr *lockedBuffer) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s := stderr.String()
		for _, line := range strings.Split(s, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "syslog udp listen ") {
				return strings.TrimPrefix(line, "syslog udp listen ")
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for udp listen; stderr=%q", stderr.String())
	return ""
}
