package capabilities

import "strings"

// Flag is a capability row class from docs/05.
type Flag string

const (
	RESTOnlyProtocol Flag = "REST_ONLY_PROTOCOL"
	ParityRequired   Flag = "PARITY_REQUIRED"
)

// Row is one frozen REST↔MCP operation.
type Row struct {
	ID          string `json:"id"`
	RESTMethod  string `json:"restMethod"`
	RESTPath    string `json:"restPath"`
	MCPTool     string `json:"mcpTool,omitempty"`
	MCPResource string `json:"mcpResource,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Flags       []Flag `json:"flags"`
	Mutating    bool   `json:"mutating"`
	Idempotent  bool   `json:"idempotent"`
}

// Table is the frozen 1.0 registry (AGENTS.md). Adding a row is a schema
// change; renaming is an ADR. HTTP adapters land in API-001 / MCP-001.
func Table() []Row {
	out := make([]Row, len(table))
	copy(out, table)
	return out
}

var table = []Row{
	{ID: "health.live", RESTMethod: "GET", RESTPath: "/v1/health/live", Flags: []Flag{RESTOnlyProtocol}, Idempotent: true},
	{ID: "health.ready", RESTMethod: "GET", RESTPath: "/v1/health/ready", Flags: []Flag{RESTOnlyProtocol}, Idempotent: true},
	{ID: "session.create", RESTMethod: "POST", RESTPath: "/v1/session", Flags: []Flag{RESTOnlyProtocol}, Mutating: true},
	{ID: "session.get", RESTMethod: "GET", RESTPath: "/v1/session", Flags: []Flag{RESTOnlyProtocol}, Idempotent: true},
	{ID: "session.delete", RESTMethod: "DELETE", RESTPath: "/v1/session", Flags: []Flag{RESTOnlyProtocol}, Mutating: true, Idempotent: true},
	{ID: "metrics.scrape", RESTMethod: "GET", RESTPath: "/v1/metrics", Flags: []Flag{RESTOnlyProtocol}, Idempotent: true},
	{ID: "ui.static", RESTMethod: "GET", RESTPath: "/", Flags: []Flag{RESTOnlyProtocol}, Idempotent: true},
	{ID: "events.stream", RESTMethod: "GET", RESTPath: "/v1/events/stream", Flags: []Flag{RESTOnlyProtocol}, Scope: "syslog.read", Idempotent: true},

	{ID: "version.get", RESTMethod: "GET", RESTPath: "/v1/version", MCPTool: "syslog_version_get", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "capabilities.get", RESTMethod: "GET", RESTPath: "/v1/capabilities", MCPTool: "syslog_capabilities_get", MCPResource: "labsyslog://capabilities", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "status.get", RESTMethod: "GET", RESTPath: "/v1/status", MCPTool: "syslog_status_get", MCPResource: "labsyslog://status", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "schema.config.get", RESTMethod: "GET", RESTPath: "/v1/schema/config", MCPTool: "syslog_schema_get", MCPResource: "labsyslog://schema/config", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "features.list", RESTMethod: "GET", RESTPath: "/v1/features", MCPTool: "syslog_features_list", MCPResource: "labsyslog://features", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "state.get", RESTMethod: "GET", RESTPath: "/v1/state", MCPTool: "syslog_state_get", MCPResource: "labsyslog://state", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "state.validate", RESTMethod: "POST", RESTPath: "/v1/state:validate", MCPTool: "syslog_state_validate", Scope: "syslog.admin", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "state.export", RESTMethod: "GET", RESTPath: "/v1/state:export", MCPTool: "syslog_state_export", Scope: "syslog.admin", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "state.reset", RESTMethod: "POST", RESTPath: "/v1/state:reset", MCPTool: "syslog_state_reset", Scope: "syslog.admin", Flags: []Flag{ParityRequired}, Mutating: true},
	{ID: "change.plan", RESTMethod: "POST", RESTPath: "/v1/changes:plan", MCPTool: "syslog_change_plan", Scope: "syslog.admin", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "change.apply", RESTMethod: "POST", RESTPath: "/v1/changes:apply", MCPTool: "syslog_change_apply", Scope: "syslog.admin", Flags: []Flag{ParityRequired}, Mutating: true},
	{ID: "messages.list", RESTMethod: "GET", RESTPath: "/v1/messages", MCPTool: "syslog_messages_list", MCPResource: "labsyslog://messages", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "message.get", RESTMethod: "GET", RESTPath: "/v1/messages/{id}", MCPTool: "syslog_message_get", MCPResource: "labsyslog://messages/{id}", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "message.raw.get", RESTMethod: "GET", RESTPath: "/v1/messages/{id}/raw", MCPTool: "syslog_message_raw_get", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "message.delete", RESTMethod: "DELETE", RESTPath: "/v1/messages/{id}", MCPTool: "syslog_message_delete", Scope: "syslog.write", Flags: []Flag{ParityRequired}, Mutating: true},
	{ID: "messages.clear", RESTMethod: "POST", RESTPath: "/v1/messages:clear", MCPTool: "syslog_messages_clear", Scope: "syslog.write", Flags: []Flag{ParityRequired}, Mutating: true},
	{ID: "messages.wait", RESTMethod: "POST", RESTPath: "/v1/messages:wait", MCPTool: "syslog_messages_wait", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "stats.get", RESTMethod: "GET", RESTPath: "/v1/stats", MCPTool: "syslog_stats_get", MCPResource: "labsyslog://stats", Scope: "syslog.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "audit.query", RESTMethod: "GET", RESTPath: "/v1/audit", MCPTool: "syslog_audit_query", MCPResource: "labsyslog://audit", Scope: "syslog.audit.read", Flags: []Flag{ParityRequired}, Idempotent: true},
	{ID: "audit.get", RESTMethod: "GET", RESTPath: "/v1/audit/{id}", MCPTool: "syslog_audit_get", Scope: "syslog.audit.read", Flags: []Flag{ParityRequired}, Idempotent: true},
}

// Tools returns PARITY_REQUIRED MCP tool names in table order.
func Tools() []string {
	var out []string
	for _, row := range table {
		if row.MCPTool != "" {
			out = append(out, row.MCPTool)
		}
	}
	return out
}

// Resources returns unique MCP resource URIs in table order.
func Resources() []string {
	seen := map[string]bool{}
	var out []string
	for _, row := range table {
		if row.MCPResource == "" || seen[row.MCPResource] {
			continue
		}
		seen[row.MCPResource] = true
		out = append(out, row.MCPResource)
	}
	return out
}

// MutatingTools returns PARITY_REQUIRED mutating MCP tool names in table order.
func MutatingTools() []string {
	var out []string
	for _, row := range table {
		if row.Mutating && row.MCPTool != "" {
			out = append(out, row.MCPTool)
		}
	}
	return out
}

// HealthNotTools returns REST_ONLY_PROTOCOL capability IDs (not MCP tools).
func HealthNotTools() []string {
	var out []string
	for _, row := range table {
		for _, f := range row.Flags {
			if f == RESTOnlyProtocol {
				out = append(out, row.ID)
				break
			}
		}
	}
	return out
}

// LookupTool returns the frozen row for an MCP tool name.
func LookupTool(name string) (Row, bool) {
	for _, row := range table {
		if row.MCPTool == name {
			return row, true
		}
	}
	return Row{}, false
}

// LookupResource returns the frozen row whose MCP resource matches uri
// (including {id} templates).
func LookupResource(uri string) (Row, bool) {
	for _, row := range table {
		if row.MCPResource == "" {
			continue
		}
		if resourceMatch(row.MCPResource, uri) {
			return row, true
		}
	}
	return Row{}, false
}

func resourceMatch(pattern, uri string) bool {
	if pattern == uri {
		return true
	}
	pParts := strings.Split(pattern, "/")
	uParts := strings.Split(uri, "/")
	if len(pParts) != len(uParts) {
		return false
	}
	for i := range pParts {
		seg := pParts[i]
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			if uParts[i] == "" {
				return false
			}
			continue
		}
		if seg != uParts[i] {
			return false
		}
	}
	return true
}
