package syslogserver

import (
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
)

// CIDRAdmission is the CIDR allow-list and datagram rate gate.
// Empty prefixes deny all. IPv4-mapped IPv6 remotes are unmapped first.
type CIDRAdmission struct {
	prefixes  []netip.Prefix
	maxPerSec int
	maxPerIP  int
	Now       func() time.Time

	mu     sync.Mutex
	window int64
	total  int
	perIP  map[netip.Addr]int
}

// NewCIDRAdmission compiles spec.admission. Invalid CIDRs fail closed.
func NewCIDRAdmission(spec model.Admission) (*CIDRAdmission, error) {
	prefixes := make([]netip.Prefix, 0, len(spec.AllowClientCIDRs))
	for _, s := range spec.AllowClientCIDRs {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, fmt.Errorf("allowClientCidrs %q: %w", s, err)
		}
		prefixes = append(prefixes, p)
	}
	return &CIDRAdmission{
		prefixes:  prefixes,
		maxPerSec: spec.MaxDatagramsPerSec,
		maxPerIP:  spec.MaxDatagramsPerIP,
		perIP:     make(map[netip.Addr]int),
	}, nil
}

// Allow reports whether remote may enter the pipeline after size/framing.
func (a *CIDRAdmission) Allow(remote netip.Addr) (bool, string) {
	if a == nil {
		return true, ""
	}
	remote = remote.Unmap()
	if !a.cidrOK(remote) {
		return false, ReasonAdmission
	}
	if !a.rateOK(remote) {
		return false, ReasonAdmissionRate
	}
	return true, ""
}

func (a *CIDRAdmission) cidrOK(remote netip.Addr) bool {
	for _, p := range a.prefixes {
		if p.Contains(remote) {
			return true
		}
	}
	return false
}

func (a *CIDRAdmission) rateOK(remote netip.Addr) bool {
	if a.maxPerSec <= 0 && a.maxPerIP <= 0 {
		return true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now().Unix()
	if now != a.window {
		a.window = now
		a.total = 0
		a.perIP = make(map[netip.Addr]int)
	}
	if a.maxPerSec > 0 && a.total >= a.maxPerSec {
		return false
	}
	if a.maxPerIP > 0 && a.perIP[remote] >= a.maxPerIP {
		return false
	}
	a.total++
	a.perIP[remote]++
	return true
}

func (a *CIDRAdmission) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}
