package syslogtest

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/store"
)

// DefaultWait is the helper timeout when the caller passes 0.
const DefaultWait = 2 * time.Second

// Wait is store.Wait that fails the test on error. It does not leave a parked waiter.
func Wait(t testing.TB, s *store.Store, f store.ListFilter, timeout time.Duration) store.WaitResult {
	t.Helper()
	if timeout <= 0 {
		timeout = DefaultWait
	}
	res, err := s.Wait(t.Context(), f, timeout)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	return res
}

// WaitCode is store.Wait that fails unless err is the given domain code.
func WaitCode(t testing.TB, s *store.Store, f store.ListFilter, timeout time.Duration, code domainerr.Code) store.WaitResult {
	t.Helper()
	if timeout <= 0 {
		timeout = DefaultWait
	}
	res, err := s.Wait(t.Context(), f, timeout)
	if !domainerr.Is(err, code) {
		t.Fatalf("wait error = %v, want %s", err, code)
	}
	return res
}

// AssertNoWaiters fails if the store still has parked waiters.
func AssertNoWaiters(t testing.TB, s *store.Store) {
	t.Helper()
	if n := s.Stats().Waiters; n != 0 {
		t.Fatalf("waiters = %d, want 0", n)
	}
}
