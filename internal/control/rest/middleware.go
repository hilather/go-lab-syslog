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
	if r.Body != nil && limit > 0 {
		r.Body = http.MaxBytesReader(nil, r.Body, limit)
	}
	return func() { s.inflight.Add(-1) }, nil
}

type hookWriter struct {
	http.ResponseWriter
	wrote     bool
	hijack404 bool
}

func (h *hookWriter) WriteHeader(code int) {
	if h.wrote {
		return
	}
	if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
		h.hijack404 = true
		h.wrote = true
		writeProblem(h.ResponseWriter, domainerr.New(domainerr.NotFound, "no such route"))
		return
	}
	h.wrote = true
	h.ResponseWriter.WriteHeader(code)
}

func (h *hookWriter) Write(p []byte) (int, error) {
	if h.hijack404 {
		return len(p), nil
	}
	if !h.wrote {
		h.WriteHeader(http.StatusOK)
		if h.hijack404 {
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
