package syslogtest

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
)

func TestWaitHelpers(t *testing.T) {
	s := store.New(store.Config{MaxMessages: 8, MaxBytes: 1 << 16, MaxWait: time.Second})
	if _, err := s.Insert(model.Message{Parsed: model.Parsed{Message: "lab-probe"}}); err != nil {
		t.Fatal(err)
	}
	res := Wait(t, s, store.ListFilter{MessageContains: "lab-probe"}, time.Second)
	if res.Matched != store.MatchedExisting {
		t.Fatalf("matched %q", res.Matched)
	}
	AssertNoWaiters(t, s)
	_ = WaitCode(t, s, store.ListFilter{MessageContains: "absent"}, 15*time.Millisecond, domainerr.WaitTimeout)
	AssertNoWaiters(t, s)
}
