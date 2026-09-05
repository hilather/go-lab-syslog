package rest

import (
	"net/http"
	"strings"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

func publicPath(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/v1/health/live", "/v1/health/ready", "/v1/metrics":
		return true
	default:
		return false
	}
}

func (s *Server) checkOrigin(r *http.Request) error {
	if healthProbe(r) {
		return nil
	}
	var allowed []string
	if snap := s.svc.Snapshot(); snap != nil {
		allowed = snap.Document.Spec.Management.AllowedOrigins
	}
	return auth.CheckOrigin(r.Header.Get("Origin"), allowed)
}

func (s *Server) authenticate(r *http.Request) (auth.Principal, error) {
	v := s.svc.Verifier()
	hdr := strings.TrimSpace(r.Header.Get("Authorization"))
	if hdr != "" {
		return v.Authenticate(auth.Request{
			Authorization: hdr,
			RemoteAddr:    r.RemoteAddr,
		})
	}
	if c, err := r.Cookie(auth.CookieName); err == nil && c.Value != "" {
		if sess, _, ok := s.svc.Sessions().Lookup(c.Value); ok {
			return auth.PrincipalFromSession(sess), nil
		}
	}
	return auth.Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
}

func (s *Server) authorize(r *http.Request, p auth.Principal) error {
	if auth.UnsafeMethod(r.Method) && strings.TrimSpace(r.Header.Get("Authorization")) == "" {
		c, err := r.Cookie(auth.CookieName)
		if err == nil && c.Value != "" {
			if s.svc.Sessions() == nil || !s.svc.Sessions().ValidCSRF(c.Value, r.Header.Get(auth.CSRFHeader)) {
				return domainerr.New(domainerr.Forbidden, "CSRF token is missing or invalid")
			}
		}
	}
	row, ok := capFor(r.Method, r.URL.Path)
	if !ok || row.Scope == "" {
		return nil
	}
	return auth.Authorize(p, row.Scope)
}

func capFor(method, path string) (capabilities.Row, bool) {
	for _, row := range capabilities.Table() {
		if row.RESTMethod != method {
			continue
		}
		if pathMatch(row.RESTPath, path) {
			return row, true
		}
	}
	return capabilities.Row{}, false
}

func pathMatch(pattern, path string) bool {
	if pattern == path {
		return true
	}
	pParts := strings.Split(strings.Trim(pattern, "/"), "/")
	aParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pParts) != len(aParts) {
		return false
	}
	for i := range pParts {
		seg := pParts[i]
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			if aParts[i] == "" {
				return false
			}
			continue
		}
		if seg != aParts[i] {
			return false
		}
	}
	return true
}

func actorOf(r *http.Request) string {
	if p, ok := auth.PrincipalFromContext(r.Context()); ok {
		return p.ID
	}
	return ""
}
