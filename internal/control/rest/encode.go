package rest

import (
	"strconv"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
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

func parseBoolPtr(v string) (*bool, error) {
	if v == "" {
		return nil, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil, domainerr.Newf(domainerr.ValidationFailed, "invalid boolean %q", v)
	}
	return &b, nil
}

func boolVal(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}
