package rest

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

const defaultHeartbeat = 15 * time.Second

// Server is the /v1 adapter. It calls app.Service only and must not import
// internal/web or internal/control/mcp.
type Server struct {
	svc       *app.Service
	mux       *http.ServeMux
	cursor    *cursorCodec
	bucket    tokenBucket
	inflight  atomic.Int64
	heartbeat time.Duration
}

// New returns the management HTTP handler (stdlib mux, no framework).
func New(svc *app.Service) http.Handler {
	return newServer(svc)
}

func newServer(svc *app.Service) *Server {
	s := &Server{
		svc:       svc,
		mux:       http.NewServeMux(),
		cursor:    newCursorCodec(),
		heartbeat: defaultHeartbeat,
	}
	s.routes()
	return s
}

// Mount returns an HTTP server for the management listener, or nil when off.
func Mount(svc *app.Service) *http.Server {
	if svc == nil || svc.ManagementListener() == nil {
		return nil
	}
	return &http.Server{
		Handler:           New(svc),
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	release, err := s.admit(r)
	if err != nil {
		writeProblem(w, err)
		return
	}
	if release != nil {
		defer release()
	}
	hw := &hookWriter{ResponseWriter: w}
	s.mux.ServeHTTP(hw, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /v1/health/live", s.healthLive)
	s.mux.HandleFunc("GET /v1/health/ready", s.healthReady)
	s.mux.HandleFunc("GET /v1/version", s.version)
	s.mux.HandleFunc("GET /v1/capabilities", s.capabilities)
	s.mux.HandleFunc("GET /v1/status", s.status)
	s.mux.HandleFunc("GET /v1/schema/config", s.schemaConfig)
	s.mux.HandleFunc("GET /v1/features", s.features)
	s.mux.HandleFunc("GET /v1/state", s.stateGet)
	s.mux.HandleFunc("POST /v1/state:validate", s.stateValidate)
	s.mux.HandleFunc("GET /v1/state:export", s.stateExport)
	s.mux.HandleFunc("POST /v1/state:reset", s.stateReset)
	s.mux.HandleFunc("POST /v1/changes:plan", s.changePlan)
	s.mux.HandleFunc("POST /v1/changes:apply", s.changeApply)
	s.mux.HandleFunc("GET /v1/messages", s.messagesList)
	s.mux.HandleFunc("GET /v1/messages/{id}/raw", s.messageRaw)
	s.mux.HandleFunc("GET /v1/messages/{id}", s.messageGet)
	s.mux.HandleFunc("DELETE /v1/messages/{id}", s.messageDelete)
	s.mux.HandleFunc("POST /v1/messages:clear", s.messagesClear)
	s.mux.HandleFunc("POST /v1/messages:wait", s.messagesWait)
	s.mux.HandleFunc("GET /v1/stats", s.stats)
	s.mux.HandleFunc("GET /v1/audit", s.auditList)
	s.mux.HandleFunc("GET /v1/audit/{id}", s.auditGet)
	s.mux.HandleFunc("GET /v1/events/stream", s.eventsStream)
	s.mux.HandleFunc("POST /v1/session", s.sessionCreate)
	s.mux.HandleFunc("GET /v1/session", s.sessionGet)
	s.mux.HandleFunc("DELETE /v1/session", s.sessionDelete)
	s.mux.HandleFunc("GET /v1/metrics", s.metrics)
	s.mux.HandleFunc("/", s.unknown)
}

func (s *Server) unknown(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, domainerr.New(domainerr.NotFound, "no such route"))
}

func (s *Server) handle(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	writeProblem(w, err)
	return true
}
