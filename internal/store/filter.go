package store

import (
	"net/netip"
	"strings"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
)

const (
	ProtocolRFC3164 = "rfc3164"
	ProtocolRFC5424 = "rfc5424"
	TransportUDP    = "udp"
	TransportTCP    = "tcp"
)

// ListFilter is the AND of the listed predicates. Zero / nil means unrestricted.
type ListFilter struct {
	Facility        string
	Severity        string
	SeverityAtLeast string
	AppName         string
	Hostname        string
	MsgID           string
	ProcID          string
	MessageContains string
	Protocol        string
	Transport       string
	SourceCIDR      string
	After           time.Time
	Before          time.Time
	Truncated       *bool
	ParseWarning    *bool
}

// ListResult is one page of newest-first matches.
type ListResult struct {
	Items      []model.Message
	Generation uint64
	NextCursor string
}

type compiledFilter struct {
	appName         string
	hostname        string
	msgID           string
	procID          string
	messageContains string
	protocol        string
	transport       string
	after           time.Time
	before          time.Time
	truncated       *bool
	parseWarning    *bool
	facility        *uint8
	severity        *uint8
	severityAtLeast *uint8
	prefix          *netip.Prefix
}

func compileFilter(f ListFilter) (compiledFilter, error) {
	cf := compiledFilter{
		appName:         f.AppName,
		hostname:        f.Hostname,
		msgID:           f.MsgID,
		procID:          f.ProcID,
		messageContains: f.MessageContains,
		protocol:        f.Protocol,
		transport:       f.Transport,
		after:           f.After,
		before:          f.Before,
		truncated:       f.Truncated,
		parseWarning:    f.ParseWarning,
	}
	if f.Facility != "" {
		n, ok := syslogwire.LookupFacility(f.Facility)
		if !ok {
			return compiledFilter{}, domainerr.Newf(domainerr.ValidationFailed, "unknown facility %q", f.Facility)
		}
		cf.facility = &n
	}
	if f.Severity != "" {
		n, ok := syslogwire.LookupSeverity(f.Severity)
		if !ok {
			return compiledFilter{}, domainerr.Newf(domainerr.ValidationFailed, "unknown severity %q", f.Severity)
		}
		cf.severity = &n
	}
	if f.SeverityAtLeast != "" {
		n, ok := syslogwire.LookupSeverity(f.SeverityAtLeast)
		if !ok {
			return compiledFilter{}, domainerr.Newf(domainerr.ValidationFailed, "unknown severityAtLeast %q", f.SeverityAtLeast)
		}
		cf.severityAtLeast = &n
	}
	switch f.Protocol {
	case "", ProtocolRFC3164, ProtocolRFC5424:
	default:
		return compiledFilter{}, domainerr.Newf(domainerr.ValidationFailed, "protocol must be rfc3164|rfc5424, got %q", f.Protocol)
	}
	switch f.Transport {
	case "", TransportUDP, TransportTCP:
	default:
		return compiledFilter{}, domainerr.Newf(domainerr.ValidationFailed, "transport must be udp|tcp, got %q", f.Transport)
	}
	if f.SourceCIDR != "" {
		p, err := netip.ParsePrefix(f.SourceCIDR)
		if err != nil {
			return compiledFilter{}, domainerr.Newf(domainerr.ValidationFailed, "invalid sourceCidr %q", f.SourceCIDR)
		}
		cf.prefix = &p
	}
	return cf, nil
}

func (cf compiledFilter) matches(m model.Message) bool {
	if cf.facility != nil && m.Parsed.Facility != *cf.facility {
		return false
	}
	if cf.severity != nil && m.Parsed.Severity != *cf.severity {
		return false
	}
	// 0 is emerg: severityAtLeast N matches numeric severity ≤ N.
	if cf.severityAtLeast != nil && m.Parsed.Severity > *cf.severityAtLeast {
		return false
	}
	if cf.appName != "" && m.Parsed.AppName != cf.appName {
		return false
	}
	if cf.hostname != "" && m.Parsed.Hostname != cf.hostname {
		return false
	}
	if cf.msgID != "" && m.Parsed.MsgID != cf.msgID {
		return false
	}
	if cf.procID != "" && m.Parsed.ProcID != cf.procID {
		return false
	}
	if cf.messageContains != "" && !strings.Contains(m.Parsed.Message, cf.messageContains) {
		return false
	}
	switch cf.protocol {
	case ProtocolRFC5424:
		if m.Parsed.Version != 1 {
			return false
		}
	case ProtocolRFC3164:
		if m.Parsed.Version == 1 {
			return false
		}
	}
	if cf.transport != "" && m.Transport != cf.transport {
		return false
	}
	if cf.prefix != nil && !cf.prefix.Contains(m.RemoteIP.Unmap()) {
		return false
	}
	if !cf.after.IsZero() && m.ReceivedAt.Before(cf.after) {
		return false
	}
	if !cf.before.IsZero() && m.ReceivedAt.After(cf.before) {
		return false
	}
	if cf.truncated != nil && m.Truncated != *cf.truncated {
		return false
	}
	if cf.parseWarning != nil {
		has := m.ParseWarning != ""
		if has != *cf.parseWarning {
			return false
		}
	}
	return true
}

// List returns newest-first matches. cursor is the last-returned id; missing
// cursor is cursor_stale. limit ≤ 0 means no cap.
func (s *Store) List(f ListFilter, cursor string, limit int) (ListResult, error) {
	cf, err := compileFilter(f)
	if err != nil {
		return ListResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if cursor != "" {
		if _, ok := s.byID[cursor]; !ok {
			return ListResult{Generation: s.generation}, domainerr.New(domainerr.CursorStale, "cursor id is not in the store")
		}
	}

	out := ListResult{Generation: s.generation, Items: []model.Message{}}
	started := cursor == ""
	for i := len(s.items) - 1; i >= 0; i-- {
		id := s.items[i].msg.ID
		if !started {
			if id == cursor {
				started = true
			}
			continue
		}
		if !cf.matches(s.items[i].msg) {
			continue
		}
		if limit > 0 && len(out.Items) == limit {
			out.NextCursor = out.Items[len(out.Items)-1].ID
			return out, nil
		}
		out.Items = append(out.Items, cloneMessage(s.items[i].msg))
	}
	return out, nil
}
