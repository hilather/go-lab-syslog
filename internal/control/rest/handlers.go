package rest

import (
	"net/http"
	"strings"
	"time"

	"github.com/hilather/go-lab-syslog/api/jsonschema"
	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/audit"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/buildinfo"
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
)

func (s *Server) healthLive(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
}

func (s *Server) healthReady(w http.ResponseWriter, _ *http.Request) {
	if !s.svc.Ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	info := buildinfo.Current()
	rev := ""
	if snap := s.svc.Snapshot(); snap != nil {
		rev = snap.Revision
	}
	writeJSON(w, http.StatusOK, map[string]any{
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
	})
}

func (s *Server) capabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": capabilities.Table()})
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	st := s.svc.State(nil)
	storeStats := s.svc.Messages().Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"ready":    s.svc.Ready(),
		"revision": st.Revision,
		"drifted":  st.Drifted,
		"listeners": map[string]any{
			"udp":        listenerJSON(s.svc.UDPAddr() != nil, addrString(s.svc.UDPAddr())),
			"tcp":        listenerJSON(s.svc.TCPAddr() != nil, addrString(s.svc.TCPAddr())),
			"management": listenerJSON(s.svc.ManagementAddr() != "", s.svc.ManagementAddr()),
		},
		"store": storeStatsJSON(storeStats),
	})
}

func listenerJSON(bound bool, addr string) map[string]any {
	return map[string]any{"bound": bound, "address": addr}
}

func addrString(a interface{ String() string }) string {
	if a == nil {
		return ""
	}
	return a.String()
}

func (s *Server) schemaConfig(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/schema+json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonschema.Document)
}

func (s *Server) features(w http.ResponseWriter, _ *http.Request) {
	rfc3164, rfc5424 := true, true
	if snap := s.svc.Snapshot(); snap != nil {
		p := snap.Document.Spec.Syslog.Parse
		rfc3164 = boolVal(p.RFC3164, true)
		rfc5424 = boolVal(p.RFC5424, true)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tls":     false,
		"rfc3164": rfc3164,
		"rfc5424": rfc5424,
		"framing": []string{"auto", "octet-counting", "non-transparent"},
	})
}

func (s *Server) stateGet(w http.ResponseWriter, r *http.Request) {
	st := s.svc.State(r.Context())
	writeJSON(w, http.StatusOK, stateJSON(st))
}

func stateJSON(st app.StateView) map[string]any {
	return map[string]any{
		"apiVersion": st.Document.APIVersion,
		"kind":       st.Document.Kind,
		"metadata":   st.Document.Metadata,
		"spec":       st.Document.Spec,
		"revision":   st.Revision,
		"generation": st.Generation,
		"drifted":    st.Drifted,
	}
}

func (s *Server) stateValidate(w http.ResponseWriter, r *http.Request) {
	body, err := readAllBody(r)
	if s.handle(w, err) {
		return
	}
	doc, err := decodeDocument(r.Header.Get("Content-Type"), body)
	if s.handle(w, err) {
		return
	}
	if err := s.svc.Validate(r.Context(), *doc); s.handle(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}

func decodeDocument(contentType string, body []byte) (*model.Document, error) {
	media := strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch strings.ToLower(media) {
	case "application/yaml", "text/yaml", "application/x-yaml", "text/x-yaml":
		return config.DecodeYAML(body)
	case "application/json", "text/json":
		return config.DecodeJSON(body)
	case "", "*/*":
		doc, err := config.DecodeJSON(body)
		if err == nil {
			return doc, nil
		}
		return config.DecodeYAML(body)
	default:
		return config.DecodeJSON(body)
	}
}

func (s *Server) stateExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	canon, err := s.svc.Export(r.Context())
	if s.handle(w, err) {
		return
	}
	switch format {
	case "", "yaml":
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(canon)
	case "json":
		doc, err := config.DecodeYAML(canon)
		if s.handle(w, err) {
			return
		}
		writeJSON(w, http.StatusOK, doc)
	default:
		writeProblem(w, domainerr.Newf(domainerr.ValidationFailed, "format must be yaml or json, got %q", format))
	}
}

func (s *Server) stateReset(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Reset(r.Context(), actorOf(r), r.URL.Query().Get("reason")); s.handle(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, stateJSON(s.svc.State(r.Context())))
}

func (s *Server) changePlan(w http.ResponseWriter, r *http.Request) {
	var req app.PlanRequest
	if err := readJSON(r, &req); s.handle(w, err) {
		return
	}
	req.Actor = actorOf(r)
	plan, err := s.svc.Plan(r.Context(), req)
	if s.handle(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) changeApply(w http.ResponseWriter, r *http.Request) {
	var req app.ApplyRequest
	if err := readJSON(r, &req); s.handle(w, err) {
		return
	}
	if k := strings.TrimSpace(r.Header.Get("Idempotency-Key")); k != "" {
		req.IdempotencyKey = k
	}
	req.Actor = actorOf(r)
	out, err := s.svc.Apply(r.Context(), req)
	if s.handle(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) stats(w http.ResponseWriter, _ *http.Request) {
	st := s.svc.Messages().Stats()
	m := s.svc.Metrics()
	writeJSON(w, http.StatusOK, map[string]any{
		"store": storeStatsJSON(st),
		"ingest": map[string]any{
			"received":             m.Received.Load(),
			"stored":               m.Stored.Load(),
			"droppedOversize":      m.DroppedOversize.Load(),
			"droppedEmpty":         m.DroppedEmpty.Load(),
			"droppedAdmission":     m.DroppedAdmission.Load(),
			"droppedAdmissionRate": m.DroppedAdmissionRate.Load(),
			"droppedBehavior":      m.DroppedBehavior.Load(),
			"droppedFilter":        m.DroppedFilter.Load(),
			"droppedUnparseable":   m.DroppedUnparseable.Load(),
			"droppedStore":         m.DroppedStore.Load(),
			"udpOversize":          m.UDPOversize.Load(),
			"tcpFramingErrors":     m.TCPFramingErrors.Load(),
			"tcpConns":             m.TCPConns.Load(),
		},
	})
}

func storeStatsJSON(st store.Stats) map[string]any {
	return map[string]any{
		"messages":    st.Messages,
		"bytes":       st.Bytes,
		"generation":  st.Generation,
		"waiters":     st.Waiters,
		"evicted":     st.Evicted,
		"rejected":    st.Rejected,
		"maxMessages": st.MaxMessages,
		"maxBytes":    st.MaxBytes,
		"fullPolicy":  st.FullPolicy,
		"rawRetain":   st.RawRetain,
	}
}

func (s *Server) auditList(w http.ResponseWriter, _ *http.Request) {
	items := s.svc.AuditRing().List()
	out := make([]auditJSON, 0, len(items))
	for _, e := range items {
		out = append(out, auditDTO(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) auditGet(w http.ResponseWriter, r *http.Request) {
	e, err := s.svc.AuditRing().Get(r.PathValue("id"))
	if s.handle(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, auditDTO(e))
}

type auditJSON struct {
	ID        string `json:"id"`
	At        string `json:"at"`
	Actor     string `json:"actor"`
	Operation string `json:"operation"`
	Reason    string `json:"reason,omitempty"`
	Revision  string `json:"revision"`
}

func auditDTO(e audit.Event) auditJSON {
	return auditJSON{
		ID:        e.ID,
		At:        e.At.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		Actor:     e.Actor,
		Operation: e.Operation,
		Reason:    e.Reason,
		Revision:  e.Revision,
	}
}

func (s *Server) sessionCreate(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeProblem(w, domainerr.New(domainerr.Unauthorized, "authentication required"))
		return
	}
	cookie, csrf, sess, err := s.svc.Sessions().Create(p)
	if s.handle(w, err) {
		return
	}
	http.SetCookie(w, auth.NewSessionCookie(cookie, auth.CookieSecure(r), s.svc.Sessions().MaxAge()))
	writeJSON(w, http.StatusOK, sessionView{
		ID:        p.ID,
		Role:      p.Role,
		Scopes:    p.Scopes,
		CSRF:      csrf,
		ExpiresAt: s.svc.Sessions().ExpiresAt(sess).UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) sessionGet(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeProblem(w, domainerr.New(domainerr.Unauthorized, "authentication required"))
		return
	}
	out := sessionView{ID: p.ID, Role: p.Role, Scopes: p.Scopes}
	if out.Scopes == nil {
		out.Scopes = []string{}
	}
	if c, err := r.Cookie(auth.CookieName); err == nil && s.svc.Sessions() != nil {
		if sess, csrf, ok := s.svc.Sessions().Lookup(c.Value); ok {
			out.CSRF = csrf
			out.ExpiresAt = s.svc.Sessions().ExpiresAt(sess).UTC().Format(time.RFC3339Nano)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) sessionDelete(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.CookieName); err == nil && s.svc.Sessions() != nil {
		s.svc.Sessions().Delete(c.Value)
	}
	http.SetCookie(w, auth.ClearSessionCookie(auth.CookieSecure(r)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

type sessionView struct {
	ID        string   `json:"id"`
	Role      string   `json:"role"`
	Scopes    []string `json:"scopes"`
	CSRF      string   `json:"csrf,omitempty"`
	ExpiresAt string   `json:"expiresAt,omitempty"`
}

func (s *Server) metrics(w http.ResponseWriter, _ *http.Request) {
	public := false
	if snap := s.svc.Snapshot(); snap != nil {
		public = snap.Document.Spec.Observability.Metrics.PublicPath
	}
	if !public {
		writeProblem(w, domainerr.New(domainerr.NotFound, "metrics publicPath is false"))
		return
	}
	w.Header().Set("Content-Type", "application/openmetrics-text; version=1.0.0; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("# labsyslog metrics placeholder (OBS-001)\n# EOF\n"))
}
