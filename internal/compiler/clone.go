package compiler

import (
	"slices"

	"github.com/hilather/go-lab-syslog/internal/model"
)

// CloneDocument returns a deep copy so Snapshot mutation cannot alias the input.
func CloneDocument(doc model.Document) model.Document {
	out := doc
	out.Spec = cloneSpec(doc.Spec)
	return out
}

func cloneSpec(s model.Spec) model.Spec {
	out := s
	out.Listeners.UDP.Enabled = cloneBool(s.Listeners.UDP.Enabled)
	out.Listeners.TCP.Enabled = cloneBool(s.Listeners.TCP.Enabled)
	out.UI.Enabled = cloneBool(s.UI.Enabled)
	out.Auth.Tokens = slices.Clone(s.Auth.Tokens)
	out.Management.AllowedOrigins = slices.Clone(s.Management.AllowedOrigins)
	out.Syslog.Parse.RFC3164 = cloneBool(s.Syslog.Parse.RFC3164)
	out.Syslog.Parse.RFC5424 = cloneBool(s.Syslog.Parse.RFC5424)
	out.Syslog.Parse.BestEffort = cloneBool(s.Syslog.Parse.BestEffort)
	out.Admission.AllowClientCIDRs = slices.Clone(s.Admission.AllowClientCIDRs)
	out.Store.RawRetain = cloneBool(s.Store.RawRetain)
	out.Filters = cloneFilters(s.Filters)
	return out
}

func cloneFilters(in []model.Filter) []model.Filter {
	if in == nil {
		return nil
	}
	out := make([]model.Filter, len(in))
	for i, f := range in {
		out[i] = f
		out[i].Enabled = cloneBool(f.Enabled)
		out[i].Match.SourceCIDRs = slices.Clone(f.Match.SourceCIDRs)
		out[i].Match.Facilities = slices.Clone(f.Match.Facilities)
		out[i].Match.Severities = slices.Clone(f.Match.Severities)
		out[i].Match.AppNames = slices.Clone(f.Match.AppNames)
		out[i].Match.Hostnames = slices.Clone(f.Match.Hostnames)
		out[i].Match.Transports = slices.Clone(f.Match.Transports)
	}
	return out
}

func cloneBool(p *bool) *bool {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
