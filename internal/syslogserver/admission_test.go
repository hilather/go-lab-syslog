package syslogserver

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

func mustAdmission(t *testing.T, spec model.Admission) *CIDRAdmission {
	t.Helper()
	a, err := NewCIDRAdmission(spec)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestCIDRAdmissionUnmapsIPv4Mapped(t *testing.T) {
	a := mustAdmission(t, model.Admission{AllowClientCIDRs: []string{"127.0.0.0/8"}})
	mapped := netip.MustParseAddr("::ffff:127.0.0.1")
	ok, reason := a.Allow(mapped)
	if !ok || reason != "" {
		t.Fatalf("mapped loopback denied: ok=%v reason=%q", ok, reason)
	}
}

func TestCIDRAdmissionEmptyDeniesAll(t *testing.T) {
	a := mustAdmission(t, model.Admission{})
	ok, reason := a.Allow(netip.MustParseAddr("127.0.0.1"))
	if ok || reason != ReasonAdmission {
		t.Fatalf("empty list ok=%v reason=%q", ok, reason)
	}
}

func TestCIDRAdmissionMiss(t *testing.T) {
	a := mustAdmission(t, model.Admission{AllowClientCIDRs: []string{"10.99.42.0/24"}})
	ok, reason := a.Allow(netip.MustParseAddr("127.0.0.1"))
	if ok || reason != ReasonAdmission {
		t.Fatalf("outside CIDR ok=%v reason=%q", ok, reason)
	}
}

func TestCIDRAdmissionInvalidPrefix(t *testing.T) {
	_, err := NewCIDRAdmission(model.Admission{AllowClientCIDRs: []string{"not-a-cidr"}})
	if err == nil {
		t.Fatal("invalid CIDR must fail closed")
	}
}

func TestCIDRAdmissionRateGlobal(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_700_000_000, 0))
	a := mustAdmission(t, model.Admission{
		AllowClientCIDRs:   []string{"10.0.0.0/8"},
		MaxDatagramsPerSec: 2,
		MaxDatagramsPerIP:  100,
	})
	a.Now = clk.Now
	ip := netip.MustParseAddr("10.0.0.1")
	for i := 0; i < 2; i++ {
		ok, reason := a.Allow(ip)
		if !ok {
			t.Fatalf("allow %d denied: %s", i, reason)
		}
	}
	ok, reason := a.Allow(ip)
	if ok || reason != ReasonAdmissionRate {
		t.Fatalf("third datagram ok=%v reason=%q", ok, reason)
	}
	clk.Advance(time.Second)
	ok, reason = a.Allow(ip)
	if !ok {
		t.Fatalf("new window denied: %s", reason)
	}
}

func TestCIDRAdmissionRatePerIP(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_700_000_000, 0))
	a := mustAdmission(t, model.Admission{
		AllowClientCIDRs:   []string{"10.0.0.0/8"},
		MaxDatagramsPerSec: 100,
		MaxDatagramsPerIP:  1,
	})
	a.Now = clk.Now
	a1 := netip.MustParseAddr("10.0.0.1")
	a2 := netip.MustParseAddr("10.0.0.2")
	ok, reason := a.Allow(a1)
	if !ok {
		t.Fatalf("first IP denied: %s", reason)
	}
	ok, reason = a.Allow(a2)
	if !ok {
		t.Fatalf("second IP denied: %s", reason)
	}
	ok, reason = a.Allow(a1)
	if ok || reason != ReasonAdmissionRate {
		t.Fatalf("same IP second datagram ok=%v reason=%q", ok, reason)
	}
}

func TestCIDRAdmissionCIDRMissDoesNotConsumeRate(t *testing.T) {
	a := mustAdmission(t, model.Admission{
		AllowClientCIDRs:   []string{"10.0.0.0/8"},
		MaxDatagramsPerSec: 1,
		MaxDatagramsPerIP:  1,
	})
	ok, reason := a.Allow(netip.MustParseAddr("192.0.2.1"))
	if ok || reason != ReasonAdmission {
		t.Fatalf("miss ok=%v reason=%q", ok, reason)
	}
	ok, reason = a.Allow(netip.MustParseAddr("10.0.0.1"))
	if !ok {
		t.Fatalf("allow-list hit consumed by CIDR miss: %s", reason)
	}
}

func TestCIDRAdmissionConcurrentRate(t *testing.T) {
	clk := testutil.NewFakeClock(time.Unix(1_700_000_000, 0))
	a := mustAdmission(t, model.Admission{
		AllowClientCIDRs:   []string{"10.0.0.0/8"},
		MaxDatagramsPerSec: 50,
		MaxDatagramsPerIP:  50,
	})
	a.Now = clk.Now
	var wg sync.WaitGroup
	var nOK, nDeny atomicCounter
	ip := netip.MustParseAddr("10.1.2.3")
	for i := 0; i < 80; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _ := a.Allow(ip)
			if ok {
				nOK.add(1)
			} else {
				nDeny.add(1)
			}
		}()
	}
	wg.Wait()
	if nOK.n != 50 {
		t.Fatalf("allowed = %d, want 50", nOK.n)
	}
	if nDeny.n != 30 {
		t.Fatalf("denied = %d, want 30", nDeny.n)
	}
}

type atomicCounter struct {
	mu sync.Mutex
	n  int
}

func (c *atomicCounter) add(d int) {
	c.mu.Lock()
	c.n += d
	c.mu.Unlock()
}
