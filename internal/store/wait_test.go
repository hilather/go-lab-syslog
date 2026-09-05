package store_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/store"
	"github.com/hilather/go-lab-syslog/internal/syslogtest"
)

func TestWaitExisting(t *testing.T) {
	s := testStore(t, store.Config{})
	mustInsert(t, s, msg("hello"))
	res := syslogtest.Wait(t, s, store.ListFilter{MessageContains: "hello"}, time.Second)
	if res.Matched != store.MatchedExisting {
		t.Fatalf("matched %q", res.Matched)
	}
	syslogtest.AssertNoWaiters(t, s)
}

func TestWaitInsertedWakesAndDoesNotLeak(t *testing.T) {
	s := testStore(t, store.Config{})
	before := runtime.NumGoroutine()

	errc := make(chan error, 1)
	resc := make(chan store.WaitResult, 1)
	go func() {
		res, err := s.Wait(context.Background(), store.ListFilter{MessageContains: "ping"}, 2*time.Second)
		if err != nil {
			errc <- err
			return
		}
		resc <- res
	}()
	waitUntil(t, func() bool { return s.Stats().Waiters == 1 })
	mustInsert(t, s, msg("ping"))
	select {
	case err := <-errc:
		t.Fatal(err)
	case res := <-resc:
		if res.Matched != store.MatchedInserted {
			t.Fatalf("matched %q", res.Matched)
		}
		if res.Message.Parsed.Message != "ping" {
			t.Fatalf("body %q", res.Message.Parsed.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not wake")
	}
	syslogtest.AssertNoWaiters(t, s)
	waitUntil(t, func() bool { return runtime.NumGoroutine() <= before+2 })
}

func TestWaitTimeout(t *testing.T) {
	s := testStore(t, store.Config{MaxWait: time.Second})
	res := syslogtest.WaitCode(t, s, store.ListFilter{MessageContains: "nope"}, 20*time.Millisecond, domainerr.WaitTimeout)
	if res.Matched != "" {
		t.Fatalf("matched %q", res.Matched)
	}
	syslogtest.AssertNoWaiters(t, s)
}

func TestWaitWipeUnblocks(t *testing.T) {
	s := testStore(t, store.Config{})
	errc := make(chan error, 1)
	go func() {
		_, err := s.Wait(context.Background(), store.ListFilter{MessageContains: "never"}, 2*time.Second)
		errc <- err
	}()
	waitUntil(t, func() bool { return s.Stats().Waiters == 1 })
	s.Wipe()
	select {
	case err := <-errc:
		if !domainerr.Is(err, domainerr.StoreWiped) {
			t.Fatalf("err = %v, want store_wiped", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wipe did not unblock wait")
	}
	syslogtest.AssertNoWaiters(t, s)
}

func TestWaitPlusWipeNoDeadlock(t *testing.T) {
	s := testStore(t, store.Config{MaxWait: 200 * time.Millisecond})
	done := make(chan struct{})
	go func() {
		_, _ = s.Wait(context.Background(), store.ListFilter{}, 100*time.Millisecond)
		close(done)
	}()
	waitUntil(t, func() bool { return s.Stats().Waiters == 1 })
	s.Wipe()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock wait+wipe")
	}
}

func TestWaitTimeoutCappedAtMaxWait(t *testing.T) {
	s := testStore(t, store.Config{MaxWait: 30 * time.Millisecond})
	start := time.Now()
	_, err := s.Wait(context.Background(), store.ListFilter{AppName: "none"}, time.Hour)
	if !domainerr.Is(err, domainerr.WaitTimeout) {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("maxWait was not applied: %s", time.Since(start))
	}
}

func TestWaitDoesNotDelete(t *testing.T) {
	s := testStore(t, store.Config{})
	mustInsert(t, s, msg("keep"))
	_ = syslogtest.Wait(t, s, store.ListFilter{MessageContains: "keep"}, time.Second)
	if s.Stats().Messages != 1 {
		t.Fatal("wait deleted the message")
	}
}

func TestWaitContextCancel(t *testing.T) {
	s := testStore(t, store.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := s.Wait(ctx, store.ListFilter{MessageContains: "x"}, time.Second)
		errc <- err
	}()
	waitUntil(t, func() bool { return s.Stats().Waiters == 1 })
	cancel()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("expected cancel error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not unblock")
	}
	syslogtest.AssertNoWaiters(t, s)
}

func waitUntil(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met")
}
