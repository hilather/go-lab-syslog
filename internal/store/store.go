package store

import (
	"bytes"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

const (
	// PerMessageOverhead is the fixed byte charge per stored message (docs/03).
	PerMessageOverhead = 256

	DefaultMaxMessages = 10000
	DefaultMaxWait     = 60 * time.Second
	DefaultWaitTimeout = 10 * time.Second

	FullPolicyEvictOldest = "evict_oldest"
	FullPolicyReject      = "reject"

	MatchedExisting = "existing"
	MatchedInserted = "inserted"
)

// DefaultMaxBytes is 256MiB.
var DefaultMaxBytes = int(256 * model.MiB)

// Config is store caps. Zero values become product defaults (rawRetain true).
type Config struct {
	MaxMessages int
	MaxBytes    int
	FullPolicy  string
	MaxWait     time.Duration
	RawRetain   *bool
	Entropy     io.Reader
}

// ConfigFromSpec maps spec.store after compile defaults.
func ConfigFromSpec(spec model.Store) Config {
	return Config{
		MaxMessages: spec.MaxMessages,
		MaxBytes:    int(spec.MaxBytes),
		FullPolicy:  spec.FullPolicy,
		MaxWait:     time.Duration(spec.MaxWait),
		RawRetain:   spec.RawRetain,
	}
}

func (c Config) withDefaults() Config {
	if c.MaxMessages <= 0 {
		c.MaxMessages = DefaultMaxMessages
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = DefaultMaxBytes
	}
	if c.FullPolicy == "" {
		c.FullPolicy = FullPolicyEvictOldest
	}
	if c.MaxWait <= 0 {
		c.MaxWait = DefaultMaxWait
	}
	if c.RawRetain == nil {
		t := true
		c.RawRetain = &t
	}
	if c.Entropy == nil {
		c.Entropy = ulid.DefaultEntropy()
	}
	return c
}

type entry struct {
	msg  model.Message
	size int
}

// Store is the coarse-mutex ring. Do not shard in 1.0. Oldest is at index 0.
type Store struct {
	mu          sync.Mutex
	maxMessages int
	maxBytes    int
	fullPolicy  string
	maxWait     time.Duration
	rawRetain   bool
	entropy     io.Reader

	items      []entry
	byID       map[string]int
	generation uint64
	bytes      int
	waiters    []*waiter
	watchers   []*watcher
	evicted    uint64
	rejected   uint64
}

// New constructs an empty store.
func New(cfg Config) *Store {
	cfg = cfg.withDefaults()
	return &Store{
		maxMessages: cfg.MaxMessages,
		maxBytes:    cfg.MaxBytes,
		fullPolicy:  cfg.FullPolicy,
		maxWait:     cfg.MaxWait,
		rawRetain:   *cfg.RawRetain,
		entropy:     cfg.Entropy,
		byID:        make(map[string]int),
	}
}

// NewFromSpec constructs a store from compiled spec.store.
func NewFromSpec(spec model.Store) *Store {
	return New(ConfigFromSpec(spec))
}

// ApplyCaps updates live store caps (replaceStoreCaps). Existing messages
// are kept unless evict_oldest needs to shrink to the new limits.
func (s *Store) ApplyCaps(cfg Config) {
	cfg = cfg.withDefaults()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxMessages = cfg.MaxMessages
	s.maxBytes = cfg.MaxBytes
	s.fullPolicy = cfg.FullPolicy
	s.maxWait = cfg.MaxWait
	s.rawRetain = *cfg.RawRetain
	if s.fullPolicy != FullPolicyEvictOldest {
		return
	}
	var n int
	for len(s.items) > s.maxMessages || s.bytes > s.maxBytes {
		s.removeAt(0)
		n++
	}
	if n > 0 {
		s.evicted += uint64(n)
		s.generation++
	}
}

// Insert generates the ULID before locking so entropy can block without
// stalling the ring. Reject is store_full; the store never closes a connection.
func (s *Store) Insert(msg model.Message) (uint64, error) {
	now := time.Now()
	if msg.ReceivedAt.IsZero() {
		msg.ReceivedAt = now
	} else {
		now = msg.ReceivedAt
	}
	id, err := ulid.New(ulid.Timestamp(now), s.entropy)
	if err != nil {
		return 0, err
	}
	msg.ID = id.String()

	// Bill len(raw) at insert even when rawRetain drops the copy; Parsed.Message can still be large.
	size := len(msg.Raw) + PerMessageOverhead
	stored := cloneMessage(msg)

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.rawRetain {
		stored.Raw = nil
	}

	if size > s.maxBytes {
		s.rejected++
		return s.generation, domainerr.New(domainerr.StoreFull, "message larger than store maxBytes")
	}

	needEvict := len(s.items)+1 > s.maxMessages || s.bytes+size > s.maxBytes
	if needEvict && s.fullPolicy == FullPolicyReject {
		s.rejected++
		return s.generation, domainerr.New(domainerr.StoreFull, "store full")
	}
	if needEvict {
		var n int
		for len(s.items)+1 > s.maxMessages || s.bytes+size > s.maxBytes {
			s.removeAt(0)
			n++
		}
		s.evicted += uint64(n)
	}

	s.items = append(s.items, entry{msg: stored, size: size})
	s.byID[stored.ID] = len(s.items) - 1
	s.bytes += size
	s.generation++
	s.notifyInserted(stored)
	s.emitLocked(ChangeEvent{Kind: EventReceived, ID: stored.ID})
	return s.generation, nil
}

// Get returns a copy of the message. Missing id is not_found.
func (s *Store) Get(id string) (model.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.byID[id]
	if !ok {
		return model.Message{}, domainerr.New(domainerr.NotFound, "message not found")
	}
	return cloneMessage(s.items[i].msg), nil
}

// Delete removes one message. Missing id is not_found.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.byID[id]
	if !ok {
		return domainerr.New(domainerr.NotFound, "message not found")
	}
	s.removeAt(i)
	s.generation++
	s.emitLocked(ChangeEvent{Kind: EventDeleted, ID: id})
	return nil
}

// Clear is Wipe: waiters still see store_wiped. Audit is the caller's job.
func (s *Store) Clear() {
	s.Wipe()
}

// Wipe empties the ring, bumps generation, and unblocks waiters with store_wiped.
func (s *Store) Wipe() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = nil
	s.byID = make(map[string]int)
	s.bytes = 0
	s.generation++
	gen := s.generation
	for _, w := range s.waiters {
		w.ch <- waitOutcome{
			err:        domainerr.New(domainerr.StoreWiped, "store wiped"),
			generation: gen,
		}
	}
	s.waiters = nil
	s.emitLocked(ChangeEvent{Kind: EventWiped})
}

// Stats is a consistent snapshot of gauges and counters.
func (s *Store) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Stats{
		Messages:    len(s.items),
		Bytes:       s.bytes,
		Generation:  s.generation,
		Waiters:     len(s.waiters),
		Evicted:     s.evicted,
		Rejected:    s.rejected,
		MaxMessages: s.maxMessages,
		MaxBytes:    s.maxBytes,
		FullPolicy:  s.fullPolicy,
		RawRetain:   s.rawRetain,
	}
}

// Stats is store gauges and counters for ready/metrics.
type Stats struct {
	Messages    int
	Bytes       int
	Generation  uint64
	Waiters     int
	Evicted     uint64
	Rejected    uint64
	MaxMessages int
	MaxBytes    int
	FullPolicy  string
	RawRetain   bool
}

func (s *Store) removeAt(i int) {
	e := s.items[i]
	delete(s.byID, e.msg.ID)
	s.bytes -= e.size
	s.items = append(s.items[:i], s.items[i+1:]...)
	for j := i; j < len(s.items); j++ {
		s.byID[s.items[j].msg.ID] = j
	}
}

func cloneMessage(m model.Message) model.Message {
	out := m
	if m.Raw != nil {
		out.Raw = bytes.Clone(m.Raw)
	}
	if m.Tags != nil {
		out.Tags = slices.Clone(m.Tags)
	}
	if m.Parsed.Structured != nil {
		out.Parsed.Structured = slices.Clone(m.Parsed.Structured)
		for i := range out.Parsed.Structured {
			if out.Parsed.Structured[i].Params != nil {
				out.Parsed.Structured[i].Params = slices.Clone(out.Parsed.Structured[i].Params)
			}
		}
	}
	return out
}
