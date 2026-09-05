package store_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
)

func testStore(t *testing.T, cfg store.Config) *store.Store {
	t.Helper()
	if cfg.MaxMessages == 0 {
		cfg.MaxMessages = 100
	}
	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = 1 << 20
	}
	if cfg.MaxWait == 0 {
		cfg.MaxWait = time.Second
	}
	return store.New(cfg)
}

func msg(body string) model.Message {
	return model.Message{
		Transport:  store.TransportUDP,
		RemoteIP:   netip.MustParseAddr("127.0.0.1"),
		RemotePort: 12345,
		Raw:        []byte(body),
		Parsed: model.Parsed{
			PRI:      13,
			Facility: 1,
			Severity: 5,
			Hostname: "host",
			AppName:  "app",
			Message:  body,
		},
	}
}

func mustInsert(t *testing.T, s *store.Store, m model.Message) uint64 {
	t.Helper()
	gen, err := s.Insert(m)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	return gen
}

func listedBodies(t *testing.T, s *store.Store) []string {
	t.Helper()
	res, err := s.List(store.ListFilter{}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(res.Items))
	for i, m := range res.Items {
		out[i] = m.Parsed.Message
	}
	return out
}

func TestCapEvictionOrder(t *testing.T) {
	s := testStore(t, store.Config{MaxMessages: 3, FullPolicy: store.FullPolicyEvictOldest})
	mustInsert(t, s, msg("m1"))
	mustInsert(t, s, msg("m2"))
	mustInsert(t, s, msg("m3"))
	mustInsert(t, s, msg("m4"))
	got := listedBodies(t, s)
	want := []string{"m4", "m3", "m2"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("bodies = %v, want %v (oldest m1 evicted)", got, want)
	}
	st := s.Stats()
	if st.Evicted != 1 || st.Messages != 3 {
		t.Fatalf("stats %+v", st)
	}
}

func TestMaxBytesEvictsSeveralToFitCandidate(t *testing.T) {
	const (
		smallRaw = 10
		largeRaw = 300
		maxBytes = 900
	)
	s := testStore(t, store.Config{
		MaxMessages: 10,
		MaxBytes:    maxBytes,
		FullPolicy:  store.FullPolicyEvictOldest,
	})
	for _, body := range []string{"s1", "s2", "s3"} {
		m := msg(body)
		m.Raw = make([]byte, smallRaw)
		mustInsert(t, s, m)
	}
	before := s.Stats().Generation
	large := msg("large")
	large.Raw = make([]byte, largeRaw)
	after := mustInsert(t, s, large)
	if after != before+1 {
		t.Fatalf("generation %d -> %d, want +1", before, after)
	}
	got := listedBodies(t, s)
	want := []string{"large", "s3"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("bodies = %v, want %v", got, want)
	}
	st := s.Stats()
	if st.Evicted != 2 {
		t.Fatalf("evicted = %d, want 2", st.Evicted)
	}
	wantBytes := (smallRaw + store.PerMessageOverhead) + (largeRaw + store.PerMessageOverhead)
	if st.Bytes != wantBytes {
		t.Fatalf("bytes = %d, want %d", st.Bytes, wantBytes)
	}
}

func TestOversizedRejectedUnderEvictOldest(t *testing.T) {
	maxBytes := 1000
	s := testStore(t, store.Config{
		MaxMessages: 10,
		MaxBytes:    maxBytes,
		FullPolicy:  store.FullPolicyEvictOldest,
	})
	mustInsert(t, s, msg("keep"))
	raw := make([]byte, maxBytes-store.PerMessageOverhead+1)
	_, err := s.Insert(model.Message{Transport: store.TransportUDP, Raw: raw, Parsed: model.Parsed{Message: "huge"}})
	if !domainerr.Is(err, domainerr.StoreFull) {
		t.Fatalf("err = %v, want store_full", err)
	}
	st := s.Stats()
	if st.Messages != 1 || st.Rejected != 1 || st.Evicted != 0 {
		t.Fatalf("stats %+v", st)
	}
	if got := listedBodies(t, s); len(got) != 1 || got[0] != "keep" {
		t.Fatalf("kept %v", got)
	}
}

func TestRejectPolicyDoesNotEvict(t *testing.T) {
	s := testStore(t, store.Config{MaxMessages: 1, FullPolicy: store.FullPolicyReject})
	mustInsert(t, s, msg("first"))
	_, err := s.Insert(msg("second"))
	if !domainerr.Is(err, domainerr.StoreFull) {
		t.Fatalf("err = %v, want store_full", err)
	}
	if got := listedBodies(t, s); len(got) != 1 || got[0] != "first" {
		t.Fatalf("got %v", got)
	}
	if s.Stats().Rejected != 1 || s.Stats().Evicted != 0 {
		t.Fatalf("stats %+v", s.Stats())
	}
}

func TestWipeEmptiesAndBumpsGeneration(t *testing.T) {
	s := testStore(t, store.Config{})
	g1 := mustInsert(t, s, msg("a"))
	g2 := mustInsert(t, s, msg("b"))
	if g2 <= g1 {
		t.Fatalf("generation did not bump on insert: %d -> %d", g1, g2)
	}
	before := s.Stats().Generation
	s.Wipe()
	st := s.Stats()
	if st.Messages != 0 || st.Bytes != 0 {
		t.Fatalf("wipe left data %+v", st)
	}
	if st.Generation <= before {
		t.Fatalf("generation %d after wipe, before %d", st.Generation, before)
	}
	if _, err := s.Get("missing"); !domainerr.Is(err, domainerr.NotFound) {
		t.Fatalf("get after wipe: %v", err)
	}
}

func TestDeleteAndClear(t *testing.T) {
	s := testStore(t, store.Config{})
	mustInsert(t, s, msg("a"))
	mustInsert(t, s, msg("b"))
	list, err := s.List(store.ListFilter{}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	id := list.Items[0].ID
	gen := s.Stats().Generation
	if err := s.Delete(id); err != nil {
		t.Fatal(err)
	}
	if s.Stats().Generation <= gen {
		t.Fatal("delete did not bump generation")
	}
	if _, err := s.Get(id); !domainerr.Is(err, domainerr.NotFound) {
		t.Fatalf("deleted get: %v", err)
	}
	if err := s.Delete(id); !domainerr.Is(err, domainerr.NotFound) {
		t.Fatalf("second delete: %v", err)
	}
	s.Clear()
	if s.Stats().Messages != 0 {
		t.Fatal("clear left messages")
	}
}

func TestRawRetainFalseDropsRaw(t *testing.T) {
	retain := false
	s := testStore(t, store.Config{RawRetain: &retain})
	const n = 2
	raw := []byte("body")
	for i := 0; i < n; i++ {
		mustInsert(t, s, msg("body"))
	}
	list, err := s.List(store.ListFilter{}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != n {
		t.Fatal(list.Items)
	}
	if list.Items[0].Raw != nil {
		t.Fatalf("raw retained: %q", list.Items[0].Raw)
	}
	if list.Items[0].Parsed.Message != "body" {
		t.Fatalf("parsed dropped: %+v", list.Items[0].Parsed)
	}
	got, err := s.Get(list.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Raw != nil {
		t.Fatalf("get raw %q", got.Raw)
	}
	wantBytes := n * (len(raw) + store.PerMessageOverhead)
	if s.Stats().Bytes != wantBytes {
		t.Fatalf("bytes = %d, want %d (original raw billed after drop)", s.Stats().Bytes, wantBytes)
	}
}

func TestTenThousandInsertsStayUnderMaxBytes(t *testing.T) {
	s := store.New(store.Config{
		MaxMessages: store.DefaultMaxMessages,
		MaxBytes:    store.DefaultMaxBytes,
		FullPolicy:  store.FullPolicyEvictOldest,
	})
	const n = 10000
	for i := 0; i < n; i++ {
		mustInsert(t, s, msg("x"))
	}
	st := s.Stats()
	if st.Messages != n {
		t.Fatalf("messages = %d", st.Messages)
	}
	if st.Bytes > st.MaxBytes {
		t.Fatalf("bytes %d > maxBytes %d", st.Bytes, st.MaxBytes)
	}
	wantBytes := n * (len("x") + store.PerMessageOverhead)
	if st.Bytes != wantBytes {
		t.Fatalf("bytes = %d, want %d", st.Bytes, wantBytes)
	}
	mustInsert(t, s, msg("overflow"))
	st = s.Stats()
	if st.Messages != n {
		t.Fatalf("after overflow messages = %d", st.Messages)
	}
	if st.Evicted != 1 {
		t.Fatalf("evicted = %d", st.Evicted)
	}
	if st.Bytes > st.MaxBytes {
		t.Fatalf("bytes %d > maxBytes %d after overflow", st.Bytes, st.MaxBytes)
	}
}

func TestULIDAssignedAndListNewestFirst(t *testing.T) {
	s := testStore(t, store.Config{})
	mustInsert(t, s, msg("old"))
	mustInsert(t, s, msg("new"))
	res, err := s.List(store.ListFilter{}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 {
		t.Fatal(res.Items)
	}
	if res.Items[0].Parsed.Message != "new" || res.Items[1].Parsed.Message != "old" {
		t.Fatalf("order %+v", res.Items)
	}
	if res.Items[0].ID == "" || res.Items[0].ID == res.Items[1].ID {
		t.Fatalf("ids %q %q", res.Items[0].ID, res.Items[1].ID)
	}
	if res.Items[0].ID < res.Items[1].ID {
		t.Fatalf("ULIDs not time-sortable: newest %q oldest %q", res.Items[0].ID, res.Items[1].ID)
	}
}

func TestListCursorAndStale(t *testing.T) {
	s := testStore(t, store.Config{})
	mustInsert(t, s, msg("a"))
	mustInsert(t, s, msg("b"))
	mustInsert(t, s, msg("c"))
	page, err := s.List(store.ListFilter{}, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Parsed.Message != "c" || page.NextCursor == "" {
		t.Fatalf("%+v", page)
	}
	page2, err := s.List(store.ListFilter{}, page.NextCursor, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 1 || page2.Items[0].Parsed.Message != "b" {
		t.Fatalf("%+v", page2)
	}
	s.Wipe()
	_, err = s.List(store.ListFilter{}, page.NextCursor, 1)
	if !domainerr.Is(err, domainerr.CursorStale) {
		t.Fatalf("stale cursor: %v", err)
	}
}

func TestRejectDoesNotBumpGeneration(t *testing.T) {
	s := testStore(t, store.Config{MaxMessages: 1, FullPolicy: store.FullPolicyReject})
	g := mustInsert(t, s, msg("only"))
	_, err := s.Insert(msg("nope"))
	if err == nil {
		t.Fatal("expected reject")
	}
	if s.Stats().Generation != g {
		t.Fatalf("generation bumped on reject")
	}
}

func TestCloneIsolatesCaller(t *testing.T) {
	s := testStore(t, store.Config{})
	m := msg("raw")
	mustInsert(t, s, m)
	m.Raw[0] = 'X'
	list, _ := s.List(store.ListFilter{}, "", 0)
	if string(list.Items[0].Raw) != "raw" {
		t.Fatalf("caller mutated store raw: %q", list.Items[0].Raw)
	}
	list.Items[0].Raw[0] = 'Y'
	got, _ := s.Get(list.Items[0].ID)
	if string(got.Raw) != "raw" {
		t.Fatalf("list copy mutated store: %q", got.Raw)
	}
}

func TestNewFromSpec(t *testing.T) {
	retain := false
	s := store.NewFromSpec(model.Store{
		MaxMessages: 4,
		MaxBytes:    64 * model.KiB,
		FullPolicy:  store.FullPolicyReject,
		MaxWait:     model.Duration(time.Second),
		RawRetain:   &retain,
	})
	st := s.Stats()
	if st.MaxMessages != 4 || st.FullPolicy != store.FullPolicyReject || st.RawRetain {
		t.Fatalf("%+v", st)
	}
}
