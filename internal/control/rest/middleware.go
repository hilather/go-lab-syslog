package rest

import (
	"net/http"
	"sync"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

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

func healthProbe(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/v1/health/live", "/v1/health/ready":
		return true
	default:
		return false
	}
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
	// Probes stay up when wait/SSE hold maxConcurrent slots (DEP-001 healthcheck).
	if healthProbe(r) {
		return nil, nil
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

type hookWriter struct {
	http.ResponseWriter
	wrote     bool
	hijack405 bool
}

func (h *hookWriter) WriteHeader(code int) {
	if h.wrote {
		return
	}
	// Mux 405 only. Handler 404s (missing message, metrics off) keep their detail.
	if code == http.StatusMethodNotAllowed {
		h.hijack405 = true
		h.wrote = true
		writeProblem(h.ResponseWriter, domainerr.New(domainerr.NotFound, "no such route"))
		return
	}
	h.wrote = true
	h.ResponseWriter.WriteHeader(code)
}

func (h *hookWriter) Write(p []byte) (int, error) {
	if h.hijack405 {
		return len(p), nil
	}
	if !h.wrote {
		h.WriteHeader(http.StatusOK)
		if h.hijack405 {
			return len(p), nil
		}
	}
	return h.ResponseWriter.Write(p)
}

func (h *hookWriter) Flush() {
	if f, ok := h.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (h *hookWriter) Unwrap() http.ResponseWriter { return h.ResponseWriter }
