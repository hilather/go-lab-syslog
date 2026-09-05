package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/store"
)

func TestRaceInsertWaitWipe(t *testing.T) {
	s := testStore(t, store.Config{
		MaxMessages: 64,
		MaxBytes:    64 << 10,
		FullPolicy:  store.FullPolicyEvictOldest,
		MaxWait:     50 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	const workers = 16
	for i := 0; i < workers; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				_, _ = s.Insert(msg("race"))
			}
		}()
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				_, _ = s.Wait(ctx, store.ListFilter{MessageContains: "race"}, 20*time.Millisecond)
			}
		}()
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				s.Wipe()
				time.Sleep(time.Millisecond)
			}
		}()
	}
	wg.Wait()
	s.Wipe()
	if s.Stats().Waiters != 0 {
		t.Fatalf("waiters left: %d", s.Stats().Waiters)
	}
}
