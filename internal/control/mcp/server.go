package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/buildinfo"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ProtocolVersion is the only MCP revision first GA speaks (ADR 0006).
	ProtocolVersion = "2026-07-28"

	// SDKModule is the official Go SDK module path.
	SDKModule = "github.com/modelcontextprotocol/go-sdk"

	// SDKVersion is the pinned official SDK tag.
	SDKVersion = "v1.7.0"

	// DefaultPath is the Streamable HTTP mount on the management listener.
	DefaultPath = "/mcp"

	headerProtocolVersion = "Mcp-Protocol-Version"
	headerRequestID       = "X-Request-ID"
	headerOrigin          = "Origin"
	headerAuthorization   = "Authorization"
)

// Config constructs the MCP adapter.
type Config struct {
	Service            *app.Service
	AllowLegacyClients bool
	// FixedPrincipal marks stdio mode. Its id and scopes are not an
	// authorization source.
	FixedPrincipal *auth.Principal
	// StdioSecret is the bearer read from --token-file at process start.
	// Empty means HTTP, which authenticates each request.
	// The value is never logged and never included in errors or String output.
	StdioSecret string
}

// String reports the config without the startup bearer.
func (c Config) String() string {
	secret := "empty"
	if c.StdioSecret != "" {
		secret = "set"
	}
	fixed := ""
	if c.FixedPrincipal != nil {
		fixed = c.FixedPrincipal.ID
	}
	return fmt.Sprintf("Service=%t AllowLegacyClients=%t FixedPrincipal=%s StdioSecret=%s",
		c.Service != nil, c.AllowLegacyClients, fixed, secret)
}

// GoString reports the config without the startup bearer.
func (c Config) GoString() string { return c.String() }

// Server is the official-SDK adapter. Third-party MCP types do not escape it.
type Server struct {
	cfg      Config
	svc      *app.Service
	sdk      *sdk.Server
	http     *sdk.StreamableHTTPHandler
	cursor   *cursorCodec
	inflight atomic.Int64
	closed   atomic.Bool
	bucket   tokenBucket
}

type ctxKey int

const ctxPrincipal ctxKey = iota

// New builds a Server. Tools and resources come from the frozen registry.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("mcp: Service is required")
	}
	if cfg.FixedPrincipal != nil && cfg.StdioSecret == "" {
		return nil, errors.New("mcp: StdioSecret is required when FixedPrincipal is set")
	}
	if cfg.StdioSecret != "" && cfg.Service.Verifier() == nil {
		return nil, errors.New("mcp: verifier is required when StdioSecret is set")
	}
	info := buildinfo.Current()
	impl := &sdk.Implementation{
		Name:    "labsyslog",
		Title:   "LabSyslog",
		Version: info.Version,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	s := &Server{
		cfg:    cfg,
		svc:    cfg.Service,
		cursor: newCursorCodec(),
	}
	sdkOpts := &sdk.ServerOptions{
		Instructions: "LabSyslog control plane. Use typed syslog_* tools; do not assume connection state. Protocol " + ProtocolVersion + ".",
		Logger:       logger,
		Capabilities: &sdk.ServerCapabilities{
			Logging:   nil,
			Tools:     &sdk.ToolCapabilities{ListChanged: false},
			Resources: &sdk.ResourceCapabilities{ListChanged: false, Subscribe: false},
		},
		SchemaCache: sdk.NewSchemaCache(),
	}
	s.sdk = sdk.NewServer(impl, sdkOpts)
	s.sdk.AddReceivingMiddleware(s.pinProtocolMiddleware)
	s.registerTools()
	s.registerResources()

	s.http = sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server {
		return s.sdk
	}, &sdk.StreamableHTTPOptions{
		Stateless:                    true,
		Logger:                       logger,
		MaxRequestBodyBytes:          -1, // enforced from the live snapshot in serveHTTP
		PropagateRequestCancellation: true,
		DisableLocalhostProtection:   true,
	})
	return s, nil
}

// Handler returns the Streamable HTTP adapter. Mount it at /mcp.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

// Close marks the adapter stopped.
func (s *Server) Close() {
	s.closed.Store(true)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	w.Header().Set(headerRequestID, reqID)

	if s.closed.Load() {
		writeRPC(w, http.StatusServiceUnavailable, domainerr.New(domainerr.ValidationFailed, "server closed"))
		return
	}

	if err := s.checkOrigin(r); err != nil {
		writeRPC(w, http.StatusForbidden, err)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeRPC(w, http.StatusMethodNotAllowed, domainerr.New(domainerr.NotFound, "method not allowed"))
		return
	}

	release, err := s.admit(r)
	if err != nil {
		status := domainerr.Lookup(domainerr.RateLimited).Status
		if de, ok := domainerr.As(err); ok {
			status = domainerr.Lookup(de.Code).Status
		}
		writeRPC(w, status, err)
		return
	}
	if release != nil {
		defer release()
	}

	defer func() {
		if rec := recover(); rec != nil {
			writeRPC(w, http.StatusInternalServerError, domainerr.New(domainerr.ValidationFailed, "internal error"))
		}
	}()

	p, err := s.authenticate(r)
	if err != nil {
		status := http.StatusUnauthorized
		if de, ok := domainerr.As(err); ok && de.Code == domainerr.Forbidden {
			status = http.StatusForbidden
		}
		writeRPC(w, status, err)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), ctxPrincipal, p))

	if !s.allowLegacy() {
		if err := validateProtocolVersion(r); err != nil {
			writeRPC(w, http.StatusBadRequest, err)
			return
		}
	}

	s.http.ServeHTTP(w, r)
}

func (s *Server) checkOrigin(r *http.Request) error {
	var allowed []string
	if snap := s.svc.Snapshot(); snap != nil {
		allowed = snap.Document.Spec.Management.AllowedOrigins
	}
	return auth.CheckOrigin(r.Header.Get(headerOrigin), allowed)
}

func (s *Server) allowLegacy() bool {
	if snap := s.svc.Snapshot(); snap != nil {
		return snap.Document.Spec.Management.MCP.AllowLegacyClients
	}
	return s.cfg.AllowLegacyClients
}

func (s *Server) admit(r *http.Request) (func(), error) {
	snap := s.svc.Snapshot()
	if snap == nil {
		return nil, domainerr.New(domainerr.ValidationFailed, "snapshot is empty")
	}
	mgmt := snap.Document.Spec.Management
	limit := int64(mgmt.BodyLimit)
	if limit > 0 && r.ContentLength > limit {
		return nil, domainerr.New(domainerr.PayloadTooLarge, "request body exceeds bodyLimit")
	}
	if r.Body != nil && limit > 0 {
		r.Body = http.MaxBytesReader(nil, r.Body, limit)
	}
	if !s.bucket.allow(float64(mgmt.RequestsPerSecond), float64(mgmt.Burst)) {
		return nil, domainerr.New(domainerr.RateLimited, "request rate exceeded")
	}
	max := int64(mgmt.MaxConcurrent)
	if max <= 0 {
		max = 256
	}
	if s.inflight.Add(1) > max {
		s.inflight.Add(-1)
		return nil, domainerr.New(domainerr.RateLimited, "too many concurrent requests")
	}
	return func() { s.inflight.Add(-1) }, nil
}

func requestID(r *http.Request) string {
	if id := r.Header.Get(headerRequestID); id != "" {
		return id
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b[:])
}

// principalFrom returns the actor for this call.
// A context principal with a non-empty id wins and is not replaced from the
// verifier. HTTP authenticates each request that way.
// The stdio path re-authenticates StdioSecret on every call. Failure is
// unauthorized and returns no principal. Success returns the live principal,
// so a demotion is seen and rotating the secret under the same id revokes
// the process. FixedPrincipal is not an authorization source.
// When this is not a stdio server and the context has no principal, the
// result is an empty principal and a nil error.
func (s *Server) principalFrom(ctx context.Context) (auth.Principal, error) {
	if ctx != nil {
		if p, ok := ctx.Value(ctxPrincipal).(auth.Principal); ok && p.ID != "" {
			return p, nil
		}
	}
	if s != nil && (s.cfg.FixedPrincipal != nil || s.cfg.StdioSecret != "") {
		var v *auth.Verifier
		if s.svc != nil {
			v = s.svc.Verifier()
		}
		p, err := v.AuthenticateBearer(s.cfg.StdioSecret)
		if err != nil {
			return auth.Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
		}
		return p, nil
	}
	return auth.Principal{}, nil
}

func (s *Server) actorOf(ctx context.Context) string {
	p, err := s.principalFrom(ctx)
	if err != nil {
		return ""
	}
	return p.ID
}

type tokenBucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func (b *tokenBucket) allow(rps, burst float64) bool {
	if rps <= 0 {
		rps = 32
	}
	if burst <= 0 {
		burst = 64
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if b.last.IsZero() {
		b.tokens = burst
		b.last = now
	} else {
		b.tokens += now.Sub(b.last).Seconds() * rps
		if b.tokens > burst {
			b.tokens = burst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
