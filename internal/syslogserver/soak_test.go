//go:build !race

package syslogserver

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

// TestSoakUDPCapsHold drives 10k UDP/s for 60s on a test bind (GA-001).
// Store caps hold; Truncated stays false; the UDP loop does not leak goroutines.
func TestSoakUDPCapsHold(t *testing.T) {
	if testing.Short() {
		t.Skip("60s 10k UDP/s soak skipped in -short")
	}

	const (
		rate        = 10000
		duration    = 60 * time.Second
		maxMessages = 1000
		maxBytes    = 1 << 20
	)

	st := store.New(store.Config{
		MaxMessages: maxMessages,
		MaxBytes:    maxBytes,
		FullPolicy:  store.FullPolicyEvictOldest,
		MaxWait:     time.Second,
	})
	h := HandlerFunc(func(_ context.Context, msg model.Message) error {
		_, err := st.Insert(msg)
		return err
	})

	before := runtime.NumGoroutine()
	srv, err := ListenUDP(testutil.Context(t), Config{
		Addr:    "127.0.0.1:0",
		Handler: h,
		Parse:   syslogwire.DefaultOptions(),
	})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	testutil.Cleanup(t, func() { _ = srv.Close() })
	if uc, ok := srv.pc.(*net.UDPConn); ok {
		_ = uc.SetReadBuffer(8 << 20)
	}

	conn, err := net.Dial("udp", srv.LocalAddr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	testutil.Cleanup(t, func() { _ = conn.Close() })
	if uc, ok := conn.(*net.UDPConn); ok {
		_ = uc.SetWriteBuffer(8 << 20)
	}

	payload := []byte("<14>Sep  4 20:52:35 soak-1 app: soak")
	end := time.Now().Add(duration)
	var sent int
	for time.Now().Before(end) {
		window := time.Now()
		for i := 0; i < rate; i++ {
			if _, err := conn.Write(payload); err != nil {
				t.Fatalf("write: %v", err)
			}
			sent++
		}
		if d := time.Second - time.Since(window); d > 0 && time.Now().Add(d).Before(end) {
			time.Sleep(d)
		}
	}
	wantSent := rate * int(duration.Seconds()) / 2
	if sent < wantSent {
		t.Fatalf("sent %d, want at least %d (10k/s for 60s)", sent, wantSent)
	}

	var last uint64
	stable := 0
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		n := srv.Metrics().Received.Load()
		if n == last && n > 0 {
			stable++
			if stable >= 5 {
				break
			}
		} else {
			stable = 0
			last = n
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := srv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	runtime.GC()

	got := srv.Metrics().Received.Load()
	wantMin := uint64(rate*int(duration.Seconds())) / 2
	if got < wantMin {
		t.Fatalf("received %d, want at least %d (10k/s soak not reaching the bind)", got, wantMin)
	}

	stats := st.Stats()
	if stats.Messages > maxMessages {
		t.Fatalf("messages %d > maxMessages %d", stats.Messages, maxMessages)
	}
	if stats.Bytes > maxBytes {
		t.Fatalf("bytes %d > maxBytes %d", stats.Bytes, maxBytes)
	}
	if got >= uint64(maxMessages) && stats.Evicted == 0 {
		t.Fatalf("expected eviction under cap after %d received; stats %+v", got, stats)
	}

	listed, err := st.List(store.ListFilter{}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range listed.Items {
		if m.Truncated {
			t.Fatal("truncated must stay false in 1.0")
		}
	}

	after := runtime.NumGoroutine()
	if after > before+8 {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	// Capped store plus runtime slack; 600k retained datagrams would be tens of MiB.
	if ms.HeapAlloc > uint64(maxBytes)*8+64<<20 {
		t.Fatalf("heap leak: HeapAlloc=%d store maxBytes=%d stats=%+v", ms.HeapAlloc, maxBytes, stats)
	}
}
