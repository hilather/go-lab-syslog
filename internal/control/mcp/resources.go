package mcp

import (
	"context"
	"strings"

	"github.com/hilather/go-lab-syslog/api/jsonschema"
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	h := s.readResource
	s.sdk.AddResource(&sdk.Resource{
		URI: "labsyslog://capabilities", Name: "capabilities",
		Description: "Capability list (same as GET /v1/capabilities).",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labsyslog://status", Name: "status",
		Description: "Ready, listeners, revision, and store stats.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labsyslog://schema/config", Name: "schema-config",
		Description: "Published v1alpha1 config JSON Schema.",
		MIMEType:    "application/schema+json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labsyslog://features", Name: "features",
		Description: "tls/rfc3164/rfc5424/framing catalog.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labsyslog://state", Name: "state",
		Description: "Redacted spec plus revision metadata (same as GET /v1/state).",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labsyslog://messages", Name: "messages",
		Description: "Stored syslog messages, newest first.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResourceTemplate(&sdk.ResourceTemplate{
		URITemplate: "labsyslog://messages/{id}", Name: "message",
		Description: "One stored message by id.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labsyslog://stats", Name: "stats",
		Description: "Store and ingest counters.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labsyslog://audit", Name: "audit",
		Description: "Mutation audit ring, newest first.",
		MIMEType:    "application/json",
	}, h)
}

func (s *Server) readResource(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, rpcError(domainerr.New(domainerr.ValidationFailed, "request canceled"))
	}
	p := s.principalFrom(ctx)
	uri := ""
	if req != nil && req.Params != nil {
		uri = req.Params.URI
	}
	if err := s.authorizeResource(p, uri); err != nil {
		return nil, rpcError(err)
	}
	body, mime, err := s.resourceBody(ctx, uri)
	if err != nil {
		return nil, rpcError(err)
	}
	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{{
			URI:      uri,
			MIMEType: mime,
			Text:     string(body),
		}},
	}, nil
}

func (s *Server) resourceBody(ctx context.Context, uri string) ([]byte, string, error) {
	switch {
	case uri == "labsyslog://capabilities":
		b, err := marshalAPI(map[string]any{"items": capabilities.Table()})
		return b, "application/json", err
	case uri == "labsyslog://status":
		b, err := marshalAPI(s.statusView())
		return b, "application/json", err
	case uri == "labsyslog://schema/config":
		return jsonschema.Document, "application/schema+json", nil
	case uri == "labsyslog://features":
		b, err := marshalAPI(s.featuresView())
		return b, "application/json", err
	case uri == "labsyslog://state":
		b, err := marshalAPI(stateJSON(s.svc.State(ctx)))
		return b, "application/json", err
	case uri == "labsyslog://messages":
		out, err := s.listMessages(messagesListIn{})
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(out)
		return b, "application/json", err
	case strings.HasPrefix(uri, "labsyslog://messages/"):
		id := strings.TrimPrefix(uri, "labsyslog://messages/")
		if id == "" || strings.Contains(id, "/") {
			return nil, "", domainerr.New(domainerr.NotFound, "not found")
		}
		msg, err := s.svc.Messages().Get(id)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(messageDTOFrom(msg, false))
		return b, "application/json", err
	case uri == "labsyslog://stats":
		b, err := marshalAPI(map[string]any{
			"store":  storeStatsJSON(s.svc.Messages().Stats()),
			"ingest": ingestJSON(s.svc.Metrics()),
		})
		return b, "application/json", err
	case uri == "labsyslog://audit":
		items := s.svc.AuditRing().List()
		out := make([]auditJSON, 0, len(items))
		for _, e := range items {
			out = append(out, auditDTO(e))
		}
		b, err := marshalAPI(map[string]any{"items": out})
		return b, "application/json", err
	default:
		return nil, "", domainerr.New(domainerr.NotFound, "not found")
	}
}
