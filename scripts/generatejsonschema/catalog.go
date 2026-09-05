package main

import (
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
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
