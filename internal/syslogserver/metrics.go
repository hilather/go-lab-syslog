package syslogserver

import "sync/atomic"

// Metrics holds ingest counters. OBS-001 exports OpenMetrics; tests read these.
type Metrics struct {
	Received             atomic.Uint64
	Stored               atomic.Uint64
	DroppedOversize      atomic.Uint64
	DroppedEmpty         atomic.Uint64
	DroppedAdmission     atomic.Uint64
	DroppedAdmissionRate atomic.Uint64
	DroppedBehavior      atomic.Uint64
	DroppedFilter        atomic.Uint64
	DroppedUnparseable   atomic.Uint64
	DroppedStore         atomic.Uint64
	UDPOversize          atomic.Uint64
	TCPFramingErrors     atomic.Uint64
	TCPConns             atomic.Int64
}

func (m *Metrics) drop(reason string) {
	if m == nil {
		return
	}
	switch reason {
	case ReasonOversize:
		m.DroppedOversize.Add(1)
	case ReasonEmpty:
		m.DroppedEmpty.Add(1)
	case ReasonAdmission:
		m.DroppedAdmission.Add(1)
	case ReasonAdmissionRate:
		m.DroppedAdmissionRate.Add(1)
	case ReasonBehavior:
		m.DroppedBehavior.Add(1)
	case ReasonFilter:
		m.DroppedFilter.Add(1)
	case ReasonUnparseable:
		m.DroppedUnparseable.Add(1)
	case ReasonStoreFull:
		m.DroppedStore.Add(1)
	}
}
