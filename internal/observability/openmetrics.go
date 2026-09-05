package observability

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Snapshot is one scrape. Ingest/store values are copied by the caller.
type Snapshot struct {
	ReceivedByTransport map[string]uint64
	StoredBy            map[string]uint64 // key transport\x1fprotocol
	DroppedByReason     map[string]uint64
	AdmissionByReason   map[string]uint64
	UDPOversize         uint64
	TCPFramingErrors    uint64
	TCPConns            int64
	StoreMessages       uint64
	StoreBytes          uint64
	StoreGeneration     uint64
	StoreEvicted        uint64
	StoreRejected       uint64
	Waiters             int64
	WaitTimeouts        uint64
	ApplyByResult       map[string]uint64
	HTTPRequests        []HTTPSample
}

// StoredKey encodes transport+protocol for StoredBy.
func StoredKey(transport, protocol string) string {
	return transport + "\x1f" + protocol
}

// WriteOpenMetrics emits OpenMetrics 1.0 text (TYPE/HELP + samples + # EOF).
func WriteOpenMetrics(w io.Writer, snap Snapshot) error {
	var b strings.Builder
	writeFamily := func(s Series, samples []omSample) {
		fmt.Fprintf(&b, "# TYPE %s %s\n", s.Name, s.Type)
		fmt.Fprintf(&b, "# HELP %s %s\n", s.Name, s.Help)
		sort.Slice(samples, func(i, j int) bool {
			if samples[i].labels != samples[j].labels {
				return samples[i].labels < samples[j].labels
			}
			return samples[i].name < samples[j].name
		})
		for _, sm := range samples {
			if sm.labels == "" {
				fmt.Fprintf(&b, "%s %s\n", sm.name, formatValue(sm.value))
				continue
			}
			fmt.Fprintf(&b, "%s{%s} %s\n", sm.name, sm.labels, formatValue(sm.value))
		}
	}

	byName := map[string]Series{}
	for _, s := range Catalog() {
		byName[s.Name] = s
	}

	recv := make([]omSample, 0, len(Transports))
	for _, t := range Transports {
		recv = append(recv, omSample{
			name:   "labsyslog_messages_received_total",
			labels: `transport="` + escapeLabel(t) + `"`,
			value:  float64(snap.ReceivedByTransport[t]),
		})
	}
	writeFamily(byName["labsyslog_messages_received_total"], recv)

	stored := make([]omSample, 0, len(Transports)*len(Protocols))
	for _, t := range Transports {
		for _, p := range Protocols {
			stored = append(stored, omSample{
				name:   "labsyslog_messages_stored_total",
				labels: `protocol="` + escapeLabel(p) + `",transport="` + escapeLabel(t) + `"`,
				value:  float64(snap.StoredBy[StoredKey(t, p)]),
			})
		}
	}
	writeFamily(byName["labsyslog_messages_stored_total"], stored)

	dropped := make([]omSample, 0, len(DropReasons))
	for _, r := range DropReasons {
		dropped = append(dropped, omSample{
			name:   "labsyslog_messages_dropped_total",
			labels: `reason="` + escapeLabel(r) + `"`,
			value:  float64(snap.DroppedByReason[r]),
		})
	}
	writeFamily(byName["labsyslog_messages_dropped_total"], dropped)

	writeFamily(byName["labsyslog_udp_oversize_total"], []omSample{{
		name: "labsyslog_udp_oversize_total", value: float64(snap.UDPOversize),
	}})
	writeFamily(byName["labsyslog_tcp_framing_errors_total"], []omSample{{
		name: "labsyslog_tcp_framing_errors_total", value: float64(snap.TCPFramingErrors),
	}})

	adm := make([]omSample, 0, len(AdmissionReasons))
	for _, r := range AdmissionReasons {
		adm = append(adm, omSample{
			name:   "labsyslog_admission_drop_total",
			labels: `reason="` + escapeLabel(r) + `"`,
			value:  float64(snap.AdmissionByReason[r]),
		})
	}
	writeFamily(byName["labsyslog_admission_drop_total"], adm)

	writeFamily(byName["labsyslog_store_messages"], []omSample{{
		name: "labsyslog_store_messages", value: float64(snap.StoreMessages),
	}})
	writeFamily(byName["labsyslog_store_bytes"], []omSample{{
		name: "labsyslog_store_bytes", value: float64(snap.StoreBytes),
	}})
	writeFamily(byName["labsyslog_store_generation"], []omSample{{
		name: "labsyslog_store_generation", value: float64(snap.StoreGeneration),
	}})
	writeFamily(byName["labsyslog_store_evicted_total"], []omSample{{
		name: "labsyslog_store_evicted_total", value: float64(snap.StoreEvicted),
	}})
	writeFamily(byName["labsyslog_store_rejected_total"], []omSample{{
		name: "labsyslog_store_rejected_total", value: float64(snap.StoreRejected),
	}})
	writeFamily(byName["labsyslog_tcp_conns"], []omSample{{
		name: "labsyslog_tcp_conns", value: float64(snap.TCPConns),
	}})
	writeFamily(byName["labsyslog_waiters"], []omSample{{
		name: "labsyslog_waiters", value: float64(snap.Waiters),
	}})
	writeFamily(byName["labsyslog_wait_timeouts_total"], []omSample{{
		name: "labsyslog_wait_timeouts_total", value: float64(snap.WaitTimeouts),
	}})

	apply := map[string]uint64{"ok": 0, "error": 0}
	for result, v := range snap.ApplyByResult {
		apply[result] = v
	}
	applySamples := make([]omSample, 0, len(apply))
	for result, v := range apply {
		applySamples = append(applySamples, omSample{
			name:   "labsyslog_apply_total",
			labels: `result="` + escapeLabel(result) + `"`,
			value:  float64(v),
		})
	}
	writeFamily(byName["labsyslog_apply_total"], applySamples)

	httpSamples := make([]omSample, 0, len(snap.HTTPRequests))
	for _, h := range snap.HTTPRequests {
		httpSamples = append(httpSamples, omSample{
			name:   "labsyslog_http_requests_total",
			labels: `code="` + escapeLabel(formatCode(h.Code)) + `",route="` + escapeLabel(h.Route) + `"`,
			value:  float64(h.Value),
		})
	}
	writeFamily(byName["labsyslog_http_requests_total"], httpSamples)

	b.WriteString("# EOF\n")
	_, err := io.WriteString(w, b.String())
	return err
}

type omSample struct {
	name   string
	labels string
	value  float64
}

func formatValue(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}
