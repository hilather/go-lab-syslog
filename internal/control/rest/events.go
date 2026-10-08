package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/store"
)

func (s *Server) eventsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		if hw, ok := w.(*hookWriter); ok {
			flusher, ok = hw.ResponseWriter.(http.Flusher)
		}
		if !ok {
			rc := http.NewResponseController(w)
			flusher = flushFunc(func() {
				_ = rc.Flush()
			})
		}
	}
	ch, cancel := s.svc.Messages().Watch()
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flush(flusher)

	hb := s.heartbeat
	if hb <= 0 {
		hb = defaultHeartbeat
	}
	ticker := time.NewTicker(hb)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !s.streamAuthorized(r) {
				return
			}
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flush(flusher)
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if !s.streamAuthorized(r) {
				return
			}
			writeSSE(w, ev)
			flush(flusher)
		}
	}
}

// streamAuthorized rechecks the credential that opened the stream.
// Verifier and View are separate critical sections and are not held
// across the select. A cookie lookup does not slide LastSeen.
func (s *Server) streamAuthorized(r *http.Request) bool {
	p, ok := s.streamPrincipal(r)
	if !ok {
		return false
	}
	return auth.Authorize(p, auth.ScopeRead) == nil
}

func (s *Server) streamPrincipal(r *http.Request) (auth.Principal, bool) {
	if hdr := strings.TrimSpace(r.Header.Get("Authorization")); hdr != "" {
		p, err := s.svc.Verifier().Authenticate(auth.Request{
			Authorization: hdr,
			RemoteAddr:    r.RemoteAddr,
		})
		if err != nil {
			return auth.Principal{}, false
		}
		return p, true
	}
	if c, err := r.Cookie(auth.CookieName); err == nil && c.Value != "" {
		sess, ok := s.svc.Sessions().View(c.Value)
		if !ok {
			return auth.Principal{}, false
		}
		return auth.PrincipalFromSession(sess), true
	}
	return auth.Principal{}, false
}

func writeSSE(w http.ResponseWriter, ev store.ChangeEvent) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, payload)
}

type flushFunc func()

func (f flushFunc) Flush() { f() }

func flush(f http.Flusher) {
	if f != nil {
		f.Flush()
	}
}
