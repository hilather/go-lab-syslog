package store

const (
	EventReceived = "syslog.received"
	EventDeleted  = "syslog.deleted"
	EventWiped    = "store.wiped"
)

// ChangeEvent is one store mutation for REST SSE (docs/06).
type ChangeEvent struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}

type watcher struct {
	ch chan ChangeEvent
}

// Watch subscribes to insert/delete/wipe. The channel is buffered; overflow
// drops events so ingest never blocks on a slow subscriber. cancel unsubscribes.
func (s *Store) Watch() (<-chan ChangeEvent, func()) {
	w := &watcher{ch: make(chan ChangeEvent, 16)}
	s.mu.Lock()
	s.watchers = append(s.watchers, w)
	s.mu.Unlock()
	var done bool
	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if done {
			return
		}
		done = true
		for i, cur := range s.watchers {
			if cur == w {
				s.watchers = append(s.watchers[:i], s.watchers[i+1:]...)
				break
			}
		}
		close(w.ch)
	}
	return w.ch, cancel
}

func (s *Store) emitLocked(ev ChangeEvent) {
	for _, w := range s.watchers {
		select {
		case w.ch <- ev:
		default:
		}
	}
}
