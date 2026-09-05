package audit

import (
	"testing"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

func TestRingAppendListGetWipe(t *testing.T) {
	r := New(2)
	a := r.Append(Event{Actor: "op", Operation: OpApply, Revision: "sha256:a"})
	_ = r.Append(Event{Actor: "op", Operation: OpPlan, Revision: "sha256:b"})
	c := r.Append(Event{Actor: "op", Operation: OpDelete, Revision: "sha256:c"})
	items := r.List()
	if len(items) != 2 {
		t.Fatalf("cap 2 kept %d", len(items))
	}
	if items[0].Operation != OpDelete || items[1].Operation != OpPlan {
		t.Fatalf("newest first: %+v", items)
	}
	got, err := r.Get(c.ID)
	if err != nil || got.ID != c.ID {
		t.Fatal(got, err)
	}
	if _, err := r.Get(a.ID); !domainerr.Is(err, domainerr.NotFound) {
		t.Fatalf("evicted id: %v", err)
	}
	r.Wipe()
	if len(r.List()) != 0 {
		t.Fatal("wipe")
	}
	if _, err := r.Get(c.ID); !domainerr.Is(err, domainerr.NotFound) {
		t.Fatalf("wiped get: %v", err)
	}
}

func TestOpsAreManagementMutations(t *testing.T) {
	want := []string{OpPlan, OpApply, OpReset, OpDelete, OpClear}
	if len(want) != 5 {
		t.Fatal(want)
	}
}
