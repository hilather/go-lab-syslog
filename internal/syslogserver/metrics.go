package syslogserver

import (
	"strings"
	"sync/atomic"

	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
)

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

	ReceivedUDP atomic.Uint64
	ReceivedTCP atomic.Uint64
	// stored[transport][protocol]: transport 0=udp 1=tcp; protocol 0=rfc3164 1=rfc5424 2=raw
	stored [2][3]atomic.Uint64
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

func (m *Metrics) receive(transport string) {
	if m == nil {
		return
	}
	m.Received.Add(1)
	switch transport {
	case TransportTCP:
		m.ReceivedTCP.Add(1)
	default:
		m.ReceivedUDP.Add(1)
	}
}

func (m *Metrics) storeMsg(msg model.Message) {
	if m == nil {
		return
	}
	m.Stored.Add(1)
	ti, pi := storedIndex(msg.Transport, protocolLabel(msg))
	m.stored[ti][pi].Add(1)
}

// ReceivedLabeled is labsyslog_messages_received_total{transport}.
func (m *Metrics) ReceivedLabeled(transport string) uint64 {
	if m == nil {
		return 0
	}
	if transport == TransportTCP {
		return m.ReceivedTCP.Load()
	}
	return m.ReceivedUDP.Load()
}

// StoredLabeled is labsyslog_messages_stored_total{transport,protocol}.
func (m *Metrics) StoredLabeled(transport, protocol string) uint64 {
	if m == nil {
		return 0
	}
	ti, pi := storedIndex(transport, protocol)
	return m.stored[ti][pi].Load()
}

func storedIndex(transport, protocol string) (int, int) {
	ti := 0
	if transport == TransportTCP {
		ti = 1
	}
	pi := 0
	switch protocol {
	case "rfc5424":
		pi = 1
	case "raw":
		pi = 2
	}
	return ti, pi
}

func protocolLabel(msg model.Message) string {
	if strings.Contains(msg.ParseWarning, syslogwire.WarnNoParser) {
		return "raw"
	}
	if msg.Parsed.Version == 1 {
		return "rfc5424"
	}
	return "rfc3164"
}
