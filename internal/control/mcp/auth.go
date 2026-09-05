package mcp

import (
	"net/http"
	"strings"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

func (s *Server) authenticate(r *http.Request) (auth.Principal, error) {
	v := s.svc.Verifier()
	h := strings.TrimSpace(r.Header.Get(headerAuthorization))
	if h != "" && strings.HasPrefix(strings.ToLower(h), "basic ") {
		return auth.Principal{}, domainerr.New(domainerr.Unauthorized, "MCP accepts bearer tokens only")
	}
	if h == "" {
		return auth.Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
	}
	return v.Authenticate(auth.Request{
		Authorization: h,
		RemoteAddr:    r.RemoteAddr,
	})
}

func (s *Server) authorizeTool(p auth.Principal, name string) error {
	row, ok := capabilities.LookupTool(name)
	if !ok || row.Scope == "" {
		return nil
	}
	return auth.Authorize(p, row.Scope)
}

func (s *Server) authorizeResource(p auth.Principal, uri string) error {
	row, ok := capabilities.LookupResource(uri)
	if !ok {
		return domainerr.New(domainerr.NotFound, "not found")
	}
	if row.Scope == "" {
		return nil
	}
	return auth.Authorize(p, row.Scope)
}
