package syslogserver

import (
	"fmt"
	"net/netip"

	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
)

// FilterClassifier is first-match classify after parse. Unmatched is capture.
type FilterClassifier struct {
	filters []compiledFilter
}

type compiledFilter struct {
	enabled         bool
	prefixes        []netip.Prefix
	facilities      map[uint8]struct{}
	severities      map[uint8]struct{}
	severityAtLeast *uint8
	appNames        map[string]struct{}
	hostnames       map[string]struct{}
	transports      map[string]struct{}
	action          string
	tag             string
}

// NewFilterClassifier compiles spec.filters in list order. Invalid match
// values fail closed. Disabled filters stay in the list and are skipped.
func NewFilterClassifier(filters []model.Filter) (*FilterClassifier, error) {
	out := make([]compiledFilter, 0, len(filters))
	for i, f := range filters {
		cf, err := compileFilter(i, f)
		if err != nil {
			return nil, err
		}
		out = append(out, cf)
	}
	return &FilterClassifier{filters: out}, nil
}

// Classify returns the first enabled match. No match is capture (ADR 0009).
func (c *FilterClassifier) Classify(msg *model.Message) (action string, tag string) {
	if c == nil || msg == nil {
		return ActionCapture, ""
	}
	for i := range c.filters {
		f := &c.filters[i]
		if !f.enabled {
			continue
		}
		if f.matches(msg) {
			return f.action, f.tag
		}
	}
	return ActionCapture, ""
}

func compileFilter(i int, f model.Filter) (compiledFilter, error) {
	cf := compiledFilter{
		enabled: f.Enabled == nil || *f.Enabled,
		action:  f.Action.Mode,
		tag:     f.Action.Tag,
	}
	if cf.action == "" {
		cf.action = ActionCapture
	}
	for _, s := range f.Match.SourceCIDRs {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return compiledFilter{}, fmt.Errorf("filters[%d].match.sourceCidrs %q: %w", i, s, err)
		}
		cf.prefixes = append(cf.prefixes, p)
	}
	if len(f.Match.Facilities) > 0 {
		cf.facilities = make(map[uint8]struct{}, len(f.Match.Facilities))
		for _, v := range f.Match.Facilities {
			n, ok := syslogwire.LookupFacility(v)
			if !ok {
				return compiledFilter{}, fmt.Errorf("filters[%d].match.facilities: unknown %q", i, v)
			}
			cf.facilities[n] = struct{}{}
		}
	}
	if len(f.Match.Severities) > 0 {
		cf.severities = make(map[uint8]struct{}, len(f.Match.Severities))
		for _, v := range f.Match.Severities {
			n, ok := syslogwire.LookupSeverity(v)
			if !ok {
				return compiledFilter{}, fmt.Errorf("filters[%d].match.severities: unknown %q", i, v)
			}
			cf.severities[n] = struct{}{}
		}
	}
	if f.Match.SeverityAtLeast != "" {
		n, ok := syslogwire.LookupSeverity(f.Match.SeverityAtLeast)
		if !ok {
			return compiledFilter{}, fmt.Errorf("filters[%d].match.severityAtLeast: unknown %q", i, f.Match.SeverityAtLeast)
		}
		cf.severityAtLeast = &n
	}
	cf.appNames = stringSet(f.Match.AppNames)
	cf.hostnames = stringSet(f.Match.Hostnames)
	cf.transports = stringSet(f.Match.Transports)
	return cf, nil
}

func (f *compiledFilter) matches(msg *model.Message) bool {
	if len(f.prefixes) > 0 {
		ip := msg.RemoteIP.Unmap()
		ok := false
		for _, p := range f.prefixes {
			if p.Contains(ip) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if f.facilities != nil {
		if _, ok := f.facilities[msg.Parsed.Facility]; !ok {
			return false
		}
	}
	if f.severities != nil {
		if _, ok := f.severities[msg.Parsed.Severity]; !ok {
			return false
		}
	}
	if f.severityAtLeast != nil && msg.Parsed.Severity > *f.severityAtLeast {
		return false
	}
	if f.appNames != nil {
		if _, ok := f.appNames[msg.Parsed.AppName]; !ok {
			return false
		}
	}
	if f.hostnames != nil {
		if _, ok := f.hostnames[msg.Parsed.Hostname]; !ok {
			return false
		}
	}
	if f.transports != nil {
		if _, ok := f.transports[msg.Transport]; !ok {
			return false
		}
	}
	return true
}

func stringSet(vals []string) map[string]struct{} {
	if len(vals) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		m[v] = struct{}{}
	}
	return m
}
