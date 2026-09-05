package rest

import (
	"net/http"
	"path"
	"strings"
)

// tryUI serves the embedded SPA after native routing misses. rest must not
// import internal/web; cmd/labsyslog wires Options.UI.
func (s *Server) tryUI(w http.ResponseWriter, r *http.Request) bool {
	if !s.spaEnabled() || reservedManagementPath(r.URL.Path) {
		return false
	}
	s.ui.ServeHTTP(w, r)
	return true
}

func (s *Server) spaEnabled() bool {
	if s == nil || s.ui == nil {
		return false
	}
	if s.uiEnabled != nil && !s.uiEnabled() {
		return false
	}
	return true
}

func (s *Server) isPublic(r *http.Request) bool {
	if publicPath(r) {
		return true
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	// SPA routes (or 404 when UI is off) must not require a bearer.
	return !reservedManagementPath(r.URL.Path)
}

// reservedManagementPath keeps /v1 and MCP off the SPA fallback so a
// missing API route stays problem+json instead of index.html.
func reservedManagementPath(p string) bool {
	p = path.Clean("/" + p)
	switch {
	case p == "/v1" || strings.HasPrefix(p, "/v1/"):
		return true
	case p == "/mcp" || strings.HasPrefix(p, "/mcp/"):
		return true
	default:
		return false
	}
}
