package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/store"
)

const (
	defaultListLimit = 50
	maxListLimit     = 500
)

func (s *Server) messagesList(w http.ResponseWriter, r *http.Request) {
	f, err := filterFromQuery(r)
	if s.handle(w, err) {
		return
	}
	limit := defaultListLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeProblem(w, domainerr.New(domainerr.ValidationFailed, "limit must be a positive integer"))
			return
		}
		limit = n
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	cursor := r.URL.Query().Get("cursor")
	inner, err := s.cursor.decode(cursor)
	if s.handle(w, err) {
		return
	}
	res, err := s.svc.Messages().List(f, inner, limit)
	if s.handle(w, err) {
		return
	}
	items := make([]messageDTO, 0, len(res.Items))
	for _, m := range res.Items {
		items = append(items, messageDTOFrom(m, false))
	}
	rev := ""
	if snap := s.svc.Snapshot(); snap != nil {
		rev = snap.Revision
	}
	out := map[string]any{
		"revision":        rev,
		"storeGeneration": res.Generation,
		"items":           items,
	}
	if res.NextCursor != "" {
		out["nextCursor"] = s.cursor.encode(res.NextCursor)
	}
	writeJSON(w, http.StatusOK, out)
}

func filterFromQuery(r *http.Request) (store.ListFilter, error) {
	q := r.URL.Query()
	trunc, err := parseBoolPtr(q.Get("truncated"))
	if err != nil {
		return store.ListFilter{}, err
	}
	warn, err := parseBoolPtr(q.Get("parseWarning"))
	if err != nil {
		return store.ListFilter{}, err
	}
	mf := messageFilter{
		Facility:        q.Get("facility"),
		Severity:        q.Get("severity"),
		SeverityAtLeast: q.Get("severityAtLeast"),
		AppName:         q.Get("appName"),
		Hostname:        q.Get("hostname"),
		MsgID:           q.Get("msgID"),
		ProcID:          q.Get("procID"),
		MessageContains: q.Get("messageContains"),
		Protocol:        q.Get("protocol"),
		Transport:       q.Get("transport"),
		SourceCIDR:      q.Get("sourceCidr"),
		After:           q.Get("after"),
		Before:          q.Get("before"),
		Truncated:       trunc,
		ParseWarning:    warn,
	}
	return mf.toStore()
}

func (s *Server) messageGet(w http.ResponseWriter, r *http.Request) {
	msg, err := s.svc.Messages().Get(r.PathValue("id"))
	if s.handle(w, err) {
		return
	}
	includeRaw := false
	if v := r.URL.Query().Get("raw"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeProblem(w, domainerr.New(domainerr.ValidationFailed, "raw must be a boolean"))
			return
		}
		includeRaw = b
	}
	writeJSON(w, http.StatusOK, messageDTOFrom(msg, includeRaw))
}

func (s *Server) messageRaw(w http.ResponseWriter, r *http.Request) {
	msg, err := s.svc.Messages().Get(r.PathValue("id"))
	if s.handle(w, err) {
		return
	}
	if len(msg.Raw) == 0 {
		writeProblem(w, domainerr.New(domainerr.NotFound, "raw bytes are not retained"))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(msg.Raw)
}

func (s *Server) messageDelete(w http.ResponseWriter, r *http.Request) {
	err := s.svc.DeleteMessage(r.Context(), r.PathValue("id"), actorOf(r), r.URL.Query().Get("reason"))
	if s.handle(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) messagesClear(w http.ResponseWriter, r *http.Request) {
	s.svc.ClearMessages(r.Context(), actorOf(r), r.URL.Query().Get("reason"))
	w.WriteHeader(http.StatusNoContent)
}

type waitRequest struct {
	Timeout string        `json:"timeout"`
	Filter  messageFilter `json:"filter"`
}

func (s *Server) messagesWait(w http.ResponseWriter, r *http.Request) {
	var req waitRequest
	body, err := readAllBody(r)
	if s.handle(w, err) {
		return
	}
	if len(bytes.TrimSpace(body)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); s.handle(w, err) {
			return
		}
	}
	f, err := req.Filter.toStore()
	if s.handle(w, err) {
		return
	}
	var timeout time.Duration
	if req.Timeout != "" {
		timeout, err = time.ParseDuration(req.Timeout)
		if err != nil {
			writeProblem(w, domainerr.Newf(domainerr.ValidationFailed, "invalid timeout %q", req.Timeout))
			return
		}
	}
	res, err := s.svc.Messages().Wait(r.Context(), f, timeout)
	if s.handle(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"matched": res.Matched,
		"message": messageDTOFrom(res.Message, false),
	})
}
