package audit

import (
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

const (
	OpPlan   = "plan"
	OpApply  = "apply"
	OpReset  = "reset"
	OpDelete = "delete"
	OpClear  = "clear"
)

// Event is one management-mutation record (docs/09). Ingest is not audited.
type Event struct {
	ID        string    `json:"id"`
	At        time.Time `json:"at"`
	Actor     string    `json:"actor"`
	Operation string    `json:"operation"`
	Reason    string    `json:"reason,omitempty"`
	Revision  string    `json:"revision"`
}

// Ring is a bounded newest-first mutation log. Reset wipes it with the store.
type Ring struct {
	mu    sync.Mutex
	cap   int
	items []Event
}

// New constructs a ring. cap <= 0 becomes 128.
func New(cap int) *Ring {
	if cap <= 0 {
		cap = 128
	}
	return &Ring{cap: cap}
}

// Append records one event. Empty ID/At are filled.
func (r *Ring) Append(e Event) Event {
	if e.ID == "" {
		e.ID = ulid.Make().String()
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	} else {
		e.At = e.At.UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, e)
	if len(r.items) > r.cap {
		r.items = r.items[len(r.items)-r.cap:]
	}
	return e
}

// List returns newest first.
func (r *Ring) List() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.items))
	for i := range r.items {
		out[len(r.items)-1-i] = r.items[i]
	}
	return out
}

// Get returns one event by id.
func (r *Ring) Get(id string) (Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.items) - 1; i >= 0; i-- {
		if r.items[i].ID == id {
			return r.items[i], nil
		}
	}
	return Event{}, domainerr.New(domainerr.NotFound, "audit event not found")
}

// Wipe drops every record. The caller may Append a reset event afterwards.
func (r *Ring) Wipe() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = nil
}

// Resize changes capacity, dropping oldest if needed.
func (r *Ring) Resize(cap int) {
	if cap <= 0 {
		cap = 128
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cap = cap
	if len(r.items) > r.cap {
		r.items = r.items[len(r.items)-r.cap:]
	}
}
