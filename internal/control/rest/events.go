package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flush(flusher)
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, ev)
			flush(flusher)
		}
	}
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
