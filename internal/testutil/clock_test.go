package testutil

import (
	"sync"
	"testing"
	"time"
)

func TestFakeClockAdvance(t *testing.T) {
	start := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	c := NewFakeClock(start)
	if !c.Now().Equal(start) {
		t.Fatal("now")
	}
	c.Advance(10 * time.Second)
	if c.Now().Sub(start) != 10*time.Second {
		t.Fatal("advance")
	}
	c.Set(start)
	if !c.Now().Equal(start) {
		t.Fatal("set")
	}
}

func TestFakeClockConcurrent(t *testing.T) {
	c := NewFakeClock(time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC))
	var wg sync.WaitGroup
	const n = 32
	wg.Add(n * 3)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = c.Now()
		}()
		go func() {
			defer wg.Done()
			c.Advance(time.Millisecond)
		}()
		go func(i int) {
			defer wg.Done()
			c.Set(time.Unix(int64(i), 0).UTC())
		}(i)
	}
	wg.Wait()
	if c.Now().IsZero() {
		t.Fatal("zero after concurrent access")
	}
}
