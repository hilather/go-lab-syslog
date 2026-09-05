package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/hilather/go-lab-syslog/api/jsonschema"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/buildinfo"
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultListLimit = 50
	maxListLimit     = 500
)

func (s *Server) registerTools() {
	addTool(s, "syslog_version_get", versionDesc, false, true, func(ctx context.Context, _ auth.Principal, _ emptyIn) (any, error) {
		info := buildinfo.Current()
		rev := ""
		if snap := s.svc.Snapshot(); snap != nil {
			rev = snap.Revision
		}
		return map[string]any{
			"version":   info.Version,
			"commit":    info.Commit,
			"buildTime": info.BuildTime,
			"module":    "github.com/hilather/go-lab-syslog",
			"revision":  rev,
			"protocols": map[string]string{
				"configAPI": info.Protocols.ConfigAPI,
				"rest":      info.Protocols.REST,
				"mcp":       info.Protocols.MCP,
			},
		}, nil
	})
	addTool(s, "syslog_capabilities_get", capDesc, false, true, func(ctx context.Context, _ auth.Principal, _ emptyIn) (any, error) {
		return map[string]any{"items": capabilities.Table()}, nil
	})
	addTool(s, "syslog_status_get", statusDesc, false, true, func(ctx context.Context, _ auth.Principal, _ emptyIn) (any, error) {
		return s.statusView(), nil
	})
	addTool(s, "syslog_schema_get", schemaDesc, false, true, func(ctx context.Context, _ auth.Principal, _ emptyIn) (any, error) {
		var doc any
		if err := json.Unmarshal(jsonschema.Document, &doc); err != nil {
			return nil, domainerr.New(domainerr.ValidationFailed, "schema unavailable")
		}
		return doc, nil
	})
	addTool(s, "syslog_features_list", featuresDesc, false, true, func(ctx context.Context, _ auth.Principal, _ emptyIn) (any, error) {
		return s.featuresView(), nil
	})
	addTool(s, "syslog_state_get", stateGetDesc, false, true, func(ctx context.Context, _ auth.Principal, _ emptyIn) (any, error) {
		return stateJSON(s.svc.State(ctx)), nil
	})
	addTool(s, "syslog_state_validate", validateDesc, false, true, func(ctx context.Context, _ auth.Principal, in validateIn) (any, error) {
		doc, err := in.document()
		if err != nil {
			return nil, err
		}
		if err := s.svc.Validate(ctx, *doc); err != nil {
			return nil, err
		}
		return map[string]any{"valid": true}, nil
	})
	addTool(s, "syslog_state_export", exportDesc, false, true, func(ctx context.Context, _ auth.Principal, in exportIn) (any, error) {
		canon, err := s.svc.Export(ctx)
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(in.Format) {
		case "", "yaml", "yml":
			return map[string]any{"format": "yaml", "body": string(canon)}, nil
		case "json":
			doc, err := config.DecodeYAML(canon)
			if err != nil {
				return nil, err
			}
			return map[string]any{"format": "json", "document": doc}, nil
		default:
			return nil, domainerr.Newf(domainerr.ValidationFailed, "format must be yaml or json, got %q", in.Format)
		}
	})
	addTool(s, "syslog_state_reset", resetDesc, true, false, func(ctx context.Context, p auth.Principal, in reasonIn) (any, error) {
		if err := s.svc.Reset(ctx, p.ID, in.Reason); err != nil {
			return nil, err
		}
		return stateJSON(s.svc.State(ctx)), nil
	})
	addTool(s, "syslog_change_plan", planDesc, false, true, func(ctx context.Context, p auth.Principal, in changeIn) (any, error) {
		plan, err := s.svc.Plan(ctx, in.planRequest(p.ID))
		if err != nil {
			return nil, err
		}
		return plan, nil
	})
	addTool(s, "syslog_change_apply", applyDesc, true, true, func(ctx context.Context, p auth.Principal, in changeIn) (any, error) {
		out, err := s.svc.Apply(ctx, in.applyRequest(p.ID))
		if err != nil {
			return nil, err
		}
		return out, nil
	})
	addTool(s, "syslog_messages_list", messagesListDesc, false, true, func(ctx context.Context, _ auth.Principal, in messagesListIn) (any, error) {
		return s.listMessages(in)
	})
	addTool(s, "syslog_message_get", messageGetDesc, false, true, func(ctx context.Context, _ auth.Principal, in messageGetIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.New(domainerr.ValidationFailed, "id is required")
		}
		msg, err := s.svc.Messages().Get(in.ID)
		if err != nil {
			return nil, err
		}
		return messageDTOFrom(msg, in.Raw), nil
	})
	addTool(s, "syslog_message_raw_get", messageRawDesc, false, true, func(ctx context.Context, _ auth.Principal, in idIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.New(domainerr.ValidationFailed, "id is required")
		}
		msg, err := s.svc.Messages().Get(in.ID)
		if err != nil {
			return nil, err
		}
		if len(msg.Raw) == 0 {
			return nil, domainerr.New(domainerr.NotFound, "raw bytes are not retained")
		}
		return map[string]any{"id": msg.ID, "raw": msg.Raw}, nil
	})
	addTool(s, "syslog_message_delete", messageDeleteDesc, true, true, func(ctx context.Context, p auth.Principal, in idIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.New(domainerr.ValidationFailed, "id is required")
		}
		if err := s.svc.DeleteMessage(ctx, in.ID, p.ID, ""); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
	addTool(s, "syslog_messages_clear", messagesClearDesc, true, false, func(ctx context.Context, p auth.Principal, in reasonIn) (any, error) {
		s.svc.ClearMessages(ctx, p.ID, in.Reason)
		return map[string]any{"ok": true}, nil
	})
	addTool(s, "syslog_messages_wait", messagesWaitDesc, false, true, func(ctx context.Context, _ auth.Principal, in waitIn) (any, error) {
		var mf messageFilter
		if in.Filter != nil {
			mf = *in.Filter
		}
		f, err := mf.toStore()
		if err != nil {
			return nil, err
		}
		var timeout time.Duration
		if in.Timeout != "" {
			timeout, err = time.ParseDuration(in.Timeout)
			if err != nil {
				return nil, domainerr.Newf(domainerr.ValidationFailed, "invalid timeout %q", in.Timeout)
			}
		}
		res, err := s.svc.Messages().Wait(ctx, f, timeout)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"matched": res.Matched,
			"message": messageDTOFrom(res.Message, false),
		}, nil
	})
	addTool(s, "syslog_stats_get", statsDesc, false, true, func(ctx context.Context, _ auth.Principal, _ emptyIn) (any, error) {
		return map[string]any{
			"store":  storeStatsJSON(s.svc.Messages().Stats()),
			"ingest": ingestJSON(s.svc.Metrics()),
		}, nil
	})
	addTool(s, "syslog_audit_query", auditQueryDesc, false, true, func(ctx context.Context, _ auth.Principal, _ emptyIn) (any, error) {
		items := s.svc.AuditRing().List()
		out := make([]auditJSON, 0, len(items))
		for _, e := range items {
			out = append(out, auditDTO(e))
		}
		return map[string]any{"items": out}, nil
	})
	addTool(s, "syslog_audit_get", auditGetDesc, false, true, func(ctx context.Context, _ auth.Principal, in idIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.New(domainerr.ValidationFailed, "id is required")
		}
		e, err := s.svc.AuditRing().Get(in.ID)
		if err != nil {
			return nil, err
		}
		return auditDTO(e), nil
	})
}

func (s *Server) statusView() map[string]any {
	st := s.svc.State(nil)
	return map[string]any{
		"ready":    s.svc.Ready(),
		"revision": st.Revision,
		"drifted":  st.Drifted,
		"listeners": map[string]any{
			"udp":        listenerJSON(s.svc.UDPAddr() != nil, addrString(s.svc.UDPAddr())),
			"tcp":        listenerJSON(s.svc.TCPAddr() != nil, addrString(s.svc.TCPAddr())),
			"management": listenerJSON(s.svc.ManagementAddr() != "", s.svc.ManagementAddr()),
		},
		"store": storeStatsJSON(s.svc.Messages().Stats()),
	}
}

func (s *Server) featuresView() map[string]any {
	rfc3164, rfc5424 := true, true
	if snap := s.svc.Snapshot(); snap != nil {
		p := snap.Document.Spec.Syslog.Parse
		rfc3164 = boolVal(p.RFC3164, true)
		rfc5424 = boolVal(p.RFC5424, true)
	}
	return map[string]any{
		"tls":     false,
		"rfc3164": rfc3164,
		"rfc5424": rfc5424,
		"framing": []string{"auto", "octet-counting", "non-transparent"},
	}
}

func (s *Server) listMessages(in messagesListIn) (any, error) {
	f, err := in.filter().toStore()
	if err != nil {
		return nil, err
	}
	limit := defaultListLimit
	if in.Limit != 0 {
		if in.Limit < 1 {
			return nil, domainerr.New(domainerr.ValidationFailed, "limit must be a positive integer")
		}
		limit = in.Limit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	inner, err := s.cursor.decode(in.Cursor)
	if err != nil {
		return nil, err
	}
	res, err := s.svc.Messages().List(f, inner, limit)
	if err != nil {
		return nil, err
	}
	items := make([]messageDTO, 0, len(res.Items))
	for _, m := range res.Items {
		items = append(items, messageDTOFrom(m, false))
	}
	rev := ""
	if snap := s.svc.Snapshot(); snap != nil {
		rev = snap.Revision
	}
	out := map[string]any{
		"revision":        rev,
		"storeGeneration": res.Generation,
		"items":           items,
	}
	if res.NextCursor != "" {
		out["nextCursor"] = s.cursor.encode(res.NextCursor)
	}
	return out, nil
}

func addTool[In any](s *Server, name, desc string, mutating, idempotent bool, h func(context.Context, auth.Principal, In) (any, error)) {
	title := name
	readOnly := !mutating
	ann := &sdk.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    readOnly,
		IdempotentHint:  idempotent,
		DestructiveHint: boolPtr(mutating && !idempotent),
		OpenWorldHint:   boolPtr(false),
	}
	sdk.AddTool(s.sdk, &sdk.Tool{
		Name:        name,
		Title:       title,
		Description: desc,
		Annotations: ann,
		InputSchema: mustInputSchema[In](name),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, any, error) {
		p := s.principalFrom(ctx)
		if err := s.authorizeTool(p, name); err != nil {
			return toolErrorResult(err), nil, nil
		}
		out, err := h(ctx, p, in)
		if err != nil {
			return toolErrorResult(err), nil, nil
		}
		structured, err := asStructured(out)
		if err != nil {
			return nil, nil, rpcError(domainerr.New(domainerr.ValidationFailed, "internal error"))
		}
		return nil, structured, nil
	})
}

func boolPtr(v bool) *bool { return &v }

const (
	versionDesc       = "Read-only. Build and protocol versions (MCP " + ProtocolVersion + ")."
	capDesc           = "Read-only. Frozen REST↔MCP capability table."
	statusDesc        = "Read-only. Ready, listeners, revision, and store stats."
	schemaDesc        = "Read-only. Published v1alpha1 config JSON Schema."
	featuresDesc      = "Read-only. tls/rfc3164/rfc5424/framing catalog."
	stateGetDesc      = "Read-only. Redacted spec plus revision metadata."
	validateDesc      = "Read-only. Validate a candidate document without writing."
	exportDesc        = "Read-only. Canonical desired-state export."
	resetDesc         = "State-changing. Reread bootstrap, wipe store and audit. Never writes the file."
	planDesc          = "Read-only dry-run. Plan operations against the live snapshot."
	applyDesc         = "State-changing. Apply operations with expectedRevision. idempotencyKey is required."
	messagesListDesc  = "Read-only. List stored syslog messages newest-first."
	messageGetDesc    = "Read-only. Get one stored message by id."
	messageRawDesc    = "Read-only. Get retained raw bytes for one message."
	messageDeleteDesc = "State-changing. Delete one stored message."
	messagesClearDesc = "State-changing. Wipe the message store (waiters see store_wiped)."
	messagesWaitDesc  = "Read-only. Block until a matching message exists or is inserted."
	statsDesc         = "Read-only. Store and ingest counters."
	auditQueryDesc    = "Read-only. Mutation audit ring, newest first."
	auditGetDesc      = "Read-only. Get one audit event by id."
)
