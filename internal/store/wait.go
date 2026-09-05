package store

import (
	"context"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

// WaitResult is one matched message. Matched is existing | inserted.
type WaitResult struct {
	Matched    string
	Message    model.Message
	Generation uint64
}

type waiter struct {
	filter compiledFilter
	ch     chan waitOutcome
}

type waitOutcome struct {
	msg        model.Message
	matched    string
	generation uint64
	err        error
}

func (o waitOutcome) result() (WaitResult, error) {
	return WaitResult{Matched: o.matched, Message: o.msg, Generation: o.generation}, o.err
}

// Wait scans newest-first, then parks under mu (subscribe-before-flush).
// timeout ≤ 0 becomes 10s and is capped at maxWait. Wipe yields store_wiped.
func (s *Store) Wait(ctx context.Context, f ListFilter, timeout time.Duration) (WaitResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cf, err := compileFilter(f)
	if err != nil {
		return WaitResult{}, err
	}
	if timeout <= 0 {
		timeout = DefaultWaitTimeout
	}

	s.mu.Lock()
	if timeout > s.maxWait {
		timeout = s.maxWait
	}
	if msg, ok := s.findNewest(cf); ok {
		gen := s.generation
		s.mu.Unlock()
		return WaitResult{Matched: MatchedExisting, Message: cloneMessage(msg), Generation: gen}, nil
	}
	if err := ctx.Err(); err != nil {
		gen := s.generation
		s.mu.Unlock()
		return WaitResult{Generation: gen}, err
	}
	w := &waiter{filter: cf, ch: make(chan waitOutcome, 1)}
	s.waiters = append(s.waiters, w)
	s.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case out := <-w.ch:
		return out.result()
	case <-ctx.Done():
		s.removeWaiter(w)
		select {
		case out := <-w.ch:
			return out.result()
		default:
			return WaitResult{Generation: s.generationLocked()}, ctx.Err()
		}
	case <-timer.C:
		s.removeWaiter(w)
		select {
		case out := <-w.ch:
			return out.result()
		default:
			return WaitResult{Generation: s.generationLocked()}, domainerr.New(domainerr.WaitTimeout, "wait timed out")
		}
	}
}

func (s *Store) findNewest(cf compiledFilter) (model.Message, bool) {
	for i := len(s.items) - 1; i >= 0; i-- {
		if cf.matches(s.items[i].msg) {
			return s.items[i].msg, true
		}
	}
	return model.Message{}, false
}

func (s *Store) notifyInserted(msg model.Message) {
	keep := s.waiters[:0]
	for _, w := range s.waiters {
		if w.filter.matches(msg) {
			w.ch <- waitOutcome{
				msg:        cloneMessage(msg),
				matched:    MatchedInserted,
				generation: s.generation,
			}
			continue
		}
		keep = append(keep, w)
	}
	s.waiters = keep
}

func (s *Store) removeWaiter(target *waiter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, w := range s.waiters {
		if w == target {
			s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
			return
		}
	}
}

func (s *Store) generationLocked() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generation
}
