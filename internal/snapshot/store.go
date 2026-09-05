package snapshot

import "sync/atomic"

// Store is an atomic.Pointer to the live Snapshot.
type Store struct {
	cur atomic.Pointer[Snapshot]
}

// Load returns the current snapshot, or nil.
func (s *Store) Load() *Snapshot {
	if s == nil {
		return nil
	}
	return s.cur.Load()
}

// Store replaces the live snapshot.
func (s *Store) Store(n *Snapshot) {
	s.cur.Store(n)
}

// Swap replaces the live snapshot and returns the previous value.
func (s *Store) Swap(n *Snapshot) *Snapshot {
	return s.cur.Swap(n)
}
