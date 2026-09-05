package observability

// ContentType is OpenMetrics 1.0 text.
const ContentType = "application/openmetrics-text; version=1.0.0; charset=utf-8"

// Series is one catalogued metric family (api/metrics/v1alpha1.json).
type Series struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Help   string   `json:"help"`
	Labels []string `json:"labels,omitempty"`
}

const (
	TypeCounter = "counter"
	TypeGauge   = "gauge"

	TransportUDP = "udp"
	TransportTCP = "tcp"

	ProtocolRFC3164 = "rfc3164"
	ProtocolRFC5424 = "rfc5424"
	ProtocolRaw     = "raw"
)

// Frozen docs/09 drop-reason label values.
var DropReasons = []string{
	"admission_cidr",
	"admission_rate",
	"filter",
	"oversize",
	"empty",
	"unparseable",
	"store_full",
	"behavior",
}

// AdmissionReasons are labsyslog_admission_drop_total label values.
var AdmissionReasons = []string{
	"admission_cidr",
	"admission_rate",
}

// Transports and Protocols are the stored/received label sets.
var (
	Transports = []string{TransportUDP, TransportTCP}
	Protocols  = []string{ProtocolRFC3164, ProtocolRFC5424, ProtocolRaw}
)

// Catalog is every series this process emits. Required docs/09 names first.
func Catalog() []Series {
	return []Series{
		{Name: "labsyslog_messages_received_total", Type: TypeCounter, Labels: []string{"transport"}, Help: "Messages that passed admission (CIDR and rate)."},
		{Name: "labsyslog_messages_stored_total", Type: TypeCounter, Labels: []string{"transport", "protocol"}, Help: "Messages inserted into the ephemeral store."},
		{Name: "labsyslog_messages_dropped_total", Type: TypeCounter, Labels: []string{"reason"}, Help: "Messages discarded before store."},
		{Name: "labsyslog_udp_oversize_total", Type: TypeCounter, Help: "UDP datagrams dropped for exceeding udpMaxDatagramBytes or maxMessageBytes."},
		{Name: "labsyslog_tcp_framing_errors_total", Type: TypeCounter, Help: "RFC 6587 framing errors that closed a TCP session."},
		{Name: "labsyslog_admission_drop_total", Type: TypeCounter, Labels: []string{"reason"}, Help: "Admission CIDR or rate drops."},
		{Name: "labsyslog_store_messages", Type: TypeGauge, Help: "Messages currently in the store."},
		{Name: "labsyslog_store_bytes", Type: TypeGauge, Help: "Billed store bytes including per-message overhead."},
		{Name: "labsyslog_store_generation", Type: TypeGauge, Help: "Store generation (insert, delete, clear, wipe)."},
		{Name: "labsyslog_store_evicted_total", Type: TypeCounter, Help: "Messages evicted under fullPolicy evict_oldest."},
		{Name: "labsyslog_store_rejected_total", Type: TypeCounter, Help: "Inserts rejected under fullPolicy reject or oversized candidates."},
		{Name: "labsyslog_tcp_conns", Type: TypeGauge, Help: "Active RFC 6587 TCP sessions."},
		{Name: "labsyslog_waiters", Type: TypeGauge, Help: "Parked messages:wait callers."},
		{Name: "labsyslog_wait_timeouts_total", Type: TypeCounter, Help: "messages:wait calls that returned wait_timeout."},
		{Name: "labsyslog_apply_total", Type: TypeCounter, Labels: []string{"result"}, Help: "changes:apply outcomes."},
		{Name: "labsyslog_http_requests_total", Type: TypeCounter, Labels: []string{"code", "route"}, Help: "Management HTTP responses. No client-IP labels."},
	}
}

// RequiredNames are the docs/09 series plus store_rejected_total from docs/03.
func RequiredNames() []string {
	return []string{
		"labsyslog_messages_received_total",
		"labsyslog_messages_stored_total",
		"labsyslog_messages_dropped_total",
		"labsyslog_udp_oversize_total",
		"labsyslog_tcp_framing_errors_total",
		"labsyslog_admission_drop_total",
		"labsyslog_store_messages",
		"labsyslog_store_bytes",
		"labsyslog_store_generation",
		"labsyslog_store_evicted_total",
		"labsyslog_store_rejected_total",
		"labsyslog_tcp_conns",
		"labsyslog_waiters",
	}
}
