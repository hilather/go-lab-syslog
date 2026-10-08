package app

import (
	"fmt"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

// Completed idempotency records are capped at 128. The oldest completed
// record is dropped. Reset clears the map.
func TestIdempotencyCacheIsBounded(t *testing.T) {
	svc := newTestService(t, "")
	ctx := testutil.Context(t)
	rev := svc.Snapshot().Revision
	original := rev
	const capN = 128
	var replay ApplyRequest
	for i := 0; i < capN+1; i++ {
		req := ApplyRequest{
			ExpectedRevision: rev,
			IdempotencyKey:   fmt.Sprintf("idem-%d", i),
			Operations:       []Operation{{Type: OpReplaceObservability, LogLevel: "debug"}},
			Actor:            "operator",
		}
		if i == capN {
			replay = req
		}
		res, err := svc.Apply(ctx, req)
		if err != nil {
			t.Fatalf("apply %d: %v", i, err)
		}
		rev = res.Revision
	}
	if got := len(svc.idem); got != capN {
		t.Fatalf("idempotency cache grew to %d entries; want %d", got, capN)
	}

	again, err := svc.Apply(ctx, replay)
	if err != nil {
		t.Fatalf("replay idem-128: %v", err)
	}
	if !again.IdempotentReplay {
		t.Fatal("replay of idem-128 was not IdempotentReplay")
	}

	_, err = svc.Apply(ctx, ApplyRequest{
		ExpectedRevision: original,
		IdempotencyKey:   "idem-0",
		Operations:       []Operation{{Type: OpReplaceObservability, LogLevel: "debug"}},
		Actor:            "operator",
	})
	if !domainerr.Is(err, domainerr.RevisionMismatch) {
		t.Fatalf("evicted idem-0: %v, want revision_mismatch", err)
	}

	if err := svc.Reset(ctx, "operator", "clear idem"); err != nil {
		t.Fatal(err)
	}
	if got := len(svc.idem); got != 0 {
		t.Fatalf("after reset len(idem)=%d, want 0", got)
	}
}
