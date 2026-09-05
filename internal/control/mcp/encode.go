package mcp

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/audit"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
	"github.com/hilather/go-lab-syslog/internal/syslogserver"
)

type messageDTO struct {
	ID           string       `json:"id"`
	ReceivedAt   time.Time    `json:"receivedAt"`
	Transport    string       `json:"transport"`
	RemoteIP     string       `json:"remoteIP,omitempty"`
	RemotePort   uint16       `json:"remotePort,omitempty"`
	Raw          []byte       `json:"raw,omitempty"`
	Truncated    bool         `json:"truncated"`
	ParseWarning string       `json:"parseWarning,omitempty"`
	Tags         []string     `json:"tags,omitempty"`
	Parsed       model.Parsed `json:"parsed"`
}

type messageFilter struct {
	Facility        string `json:"facility,omitempty"`
	Severity        string `json:"severity,omitempty"`
	SeverityAtLeast string `json:"severityAtLeast,omitempty"`
	AppName         string `json:"appName,omitempty"`
	Hostname        string `json:"hostname,omitempty"`
	MsgID           string `json:"msgID,omitempty"`
	ProcID          string `json:"procID,omitempty"`
	MessageContains string `json:"messageContains,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	Transport       string `json:"transport,omitempty"`
	SourceCIDR      string `json:"sourceCidr,omitempty"`
	After           string `json:"after,omitempty"`
	Before          string `json:"before,omitempty"`
	Truncated       *bool  `json:"truncated,omitempty"`
	ParseWarning    *bool  `json:"parseWarning,omitempty"`
}

func messageDTOFrom(m model.Message, includeRaw bool) messageDTO {
	out := messageDTO{
		ID:           m.ID,
		ReceivedAt:   m.ReceivedAt.UTC(),
		Transport:    m.Transport,
		RemotePort:   m.RemotePort,
		Truncated:    m.Truncated,
		ParseWarning: m.ParseWarning,
		Tags:         m.Tags,
		Parsed:       m.Parsed,
	}
	if m.RemoteIP.IsValid() {
		out.RemoteIP = m.RemoteIP.String()
	}
	if includeRaw {
		out.Raw = m.Raw
	}
	return out
}

func (f messageFilter) toStore() (store.ListFilter, error) {
	out := store.ListFilter{
		Facility:        f.Facility,
		Severity:        f.Severity,
		SeverityAtLeast: f.SeverityAtLeast,
		AppName:         f.AppName,
		Hostname:        f.Hostname,
		MsgID:           f.MsgID,
		ProcID:          f.ProcID,
		MessageContains: f.MessageContains,
		Protocol:        f.Protocol,
		Transport:       f.Transport,
		SourceCIDR:      f.SourceCIDR,
		Truncated:       f.Truncated,
		ParseWarning:    f.ParseWarning,
	}
	var err error
	if f.After != "" {
		out.After, err = time.Parse(time.RFC3339Nano, f.After)
		if err != nil {
			out.After, err = time.Parse(time.RFC3339, f.After)
		}
		if err != nil {
			return store.ListFilter{}, domainerr.Newf(domainerr.ValidationFailed, "invalid after %q", f.After)
		}
	}
	if f.Before != "" {
		out.Before, err = time.Parse(time.RFC3339Nano, f.Before)
		if err != nil {
			out.Before, err = time.Parse(time.RFC3339, f.Before)
		}
		if err != nil {
			return store.ListFilter{}, domainerr.Newf(domainerr.ValidationFailed, "invalid before %q", f.Before)
		}
	}
	return out, nil
}

func stateJSON(st app.StateView) map[string]any {
	return map[string]any{
		"apiVersion": st.Document.APIVersion,
		"kind":       st.Document.Kind,
		"metadata":   st.Document.Metadata,
		"spec":       st.Document.Spec,
		"revision":   st.Revision,
		"generation": st.Generation,
		"drifted":    st.Drifted,
	}
}

func storeStatsJSON(st store.Stats) map[string]any {
	return map[string]any{
		"messages":    st.Messages,
		"bytes":       st.Bytes,
		"generation":  st.Generation,
		"waiters":     st.Waiters,
		"evicted":     st.Evicted,
		"rejected":    st.Rejected,
		"maxMessages": st.MaxMessages,
		"maxBytes":    st.MaxBytes,
		"fullPolicy":  st.FullPolicy,
		"rawRetain":   st.RawRetain,
	}
}

func ingestJSON(m *syslogserver.Metrics) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return map[string]any{
		"received":             m.Received.Load(),
		"stored":               m.Stored.Load(),
		"droppedOversize":      m.DroppedOversize.Load(),
		"droppedEmpty":         m.DroppedEmpty.Load(),
		"droppedAdmission":     m.DroppedAdmission.Load(),
		"droppedAdmissionRate": m.DroppedAdmissionRate.Load(),
		"droppedBehavior":      m.DroppedBehavior.Load(),
		"droppedFilter":        m.DroppedFilter.Load(),
		"droppedUnparseable":   m.DroppedUnparseable.Load(),
		"droppedStore":         m.DroppedStore.Load(),
		"udpOversize":          m.UDPOversize.Load(),
		"tcpFramingErrors":     m.TCPFramingErrors.Load(),
		"tcpConns":             m.TCPConns.Load(),
	}
}

type auditJSON struct {
	ID        string `json:"id"`
	At        string `json:"at"`
	Actor     string `json:"actor"`
	Operation string `json:"operation"`
	Reason    string `json:"reason,omitempty"`
	Revision  string `json:"revision"`
}

func auditDTO(e audit.Event) auditJSON {
	return auditJSON{
		ID:        e.ID,
		At:        e.At.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		Actor:     e.Actor,
		Operation: e.Operation,
		Reason:    e.Reason,
		Revision:  e.Revision,
	}
}

func listenerJSON(bound bool, addr string) map[string]any {
	return map[string]any{"bound": bound, "address": addr}
}

func addrString(a interface{ String() string }) string {
	if a == nil {
		return ""
	}
	return a.String()
}

func boolVal(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func marshalAPI(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	return json.Marshal(tree)
}

func asStructured(v any) (any, error) {
	raw, err := marshalAPI(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
