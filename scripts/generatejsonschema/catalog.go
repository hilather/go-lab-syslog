package main

import (
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/observability"
)

func errorsCatalog() obj {
	codes := make([]obj, 0, len(domainerr.Codes()))
	for _, c := range domainerr.Codes() {
		info := domainerr.Lookup(c)
		codes = append(codes, obj{
			{"code", string(c)},
			{"title", info.Title},
			{"status", info.Status},
			{"type", domainerr.TypeURI(c)},
			{"urn", domainerr.URN(c)},
		})
	}
	return obj{{"codes", codes}}
}

func metricsCatalog() obj {
	series := make([]obj, 0, len(observability.Catalog()))
	for _, s := range observability.Catalog() {
		item := obj{
			{"name", s.Name},
			{"type", s.Type},
			{"help", s.Help},
		}
		if len(s.Labels) > 0 {
			item = append(item, kv{"labels", s.Labels})
		}
		series = append(series, item)
	}
	return obj{
		{"apiVersion", "labsyslog.dev/v1alpha1"},
		{"kind", "MetricsCatalog"},
		{"contentType", observability.ContentType},
		{"series", series},
	}
}

func capabilitiesCatalog() obj {
	items := make([]obj, 0, len(capabilities.Table()))
	for _, row := range capabilities.Table() {
		item := obj{
			{"id", row.ID},
			{"restMethod", row.RESTMethod},
			{"restPath", row.RESTPath},
		}
		if row.MCPTool != "" {
			item = append(item, kv{"mcpTool", row.MCPTool})
		}
		if row.MCPResource != "" {
			item = append(item, kv{"mcpResource", row.MCPResource})
		}
		if row.Scope != "" {
			item = append(item, kv{"scope", row.Scope})
		}
		flags := make([]string, 0, len(row.Flags))
		for _, f := range row.Flags {
			flags = append(flags, string(f))
		}
		item = append(item,
			kv{"flags", flags},
			kv{"mutating", row.Mutating},
			kv{"idempotent", row.Idempotent},
		)
		items = append(items, item)
	}
	return obj{{"items", items}}
}
