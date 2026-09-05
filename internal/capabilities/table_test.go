package capabilities

import (
	"testing"
)

func TestTableFrozenParity(t *testing.T) {
	seen := map[string]bool{}
	var parity, restOnly int
	for _, row := range Table() {
		if row.ID == "" || row.RESTMethod == "" || row.RESTPath == "" {
			t.Fatalf("incomplete row %+v", row)
		}
		if seen[row.ID] {
			t.Fatalf("duplicate id %s", row.ID)
		}
		seen[row.ID] = true
		hasParity := false
		hasRESTOnly := false
		for _, f := range row.Flags {
			if f == ParityRequired {
				hasParity = true
			}
			if f == RESTOnlyProtocol {
				hasRESTOnly = true
			}
		}
		if hasParity && hasRESTOnly {
			t.Fatalf("%s has both flags", row.ID)
		}
		if hasParity {
			parity++
			if row.MCPTool == "" {
				t.Fatalf("%s PARITY_REQUIRED missing MCP tool", row.ID)
			}
		}
		if hasRESTOnly {
			restOnly++
			if row.MCPTool != "" {
				t.Fatalf("%s REST_ONLY has MCP tool %s", row.ID, row.MCPTool)
			}
		}
	}
	if parity < 20 {
		t.Fatalf("parity rows = %d", parity)
	}
	if restOnly < 6 {
		t.Fatalf("rest-only rows = %d", restOnly)
	}
	for _, id := range []string{
		"state.get", "state.reset", "change.plan", "change.apply",
		"messages.wait", "health.ready", "events.stream",
	} {
		if !seen[id] {
			t.Fatalf("missing %s", id)
		}
	}
}
