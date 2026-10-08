package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

const problemType = "application/problem+json"

func writeProblem(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		err = domainerr.New(domainerr.PayloadTooLarge, "request body exceeds bodyLimit")
	}
	p := domainerr.ProblemOf(err)
	if p.Status == 0 {
		p.Status = http.StatusInternalServerError
	}
	if p.Code == domainerr.Unauthorized {
		w.Header().Set("WWW-Authenticate", auth.WWWAuthenticate())
	}
	w.Header().Set("Content-Type", problemType)
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, dst any) error {
	if r == nil || r.Body == nil {
		return domainerr.New(domainerr.ValidationFailed, "request body is required")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return domainerr.New(domainerr.ValidationFailed, "request body is required")
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return domainerr.New(domainerr.PayloadTooLarge, "request body exceeds bodyLimit")
		}
		return domainerr.New(domainerr.ValidationFailed, err.Error())
	}
	return rejectTrailingJSON(dec)
}

// rejectTrailingJSON fails when another value follows a successful decode.
// A body-limit error from the underlying reader is payload_too_large.
// Whitespace after the value is EOF and is accepted.
func rejectTrailingJSON(dec *json.Decoder) error {
	_, err := dec.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return domainerr.New(domainerr.PayloadTooLarge, "request body exceeds bodyLimit")
	}
	return domainerr.New(domainerr.ValidationFailed, "trailing JSON after document")
}

// readIgnoredJSON accepts an empty body or one JSON value and ignores that
// value. A second value, a trailing comma, or non-JSON is validation_failed.
func readIgnoredJSON(r *http.Request) error {
	if r == nil || r.Body == nil {
		return nil
	}
	buf, err := readAllBody(r)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(buf)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(buf))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return domainerr.New(domainerr.PayloadTooLarge, "request body exceeds bodyLimit")
		}
		return domainerr.New(domainerr.ValidationFailed, err.Error())
	}
	return rejectTrailingJSON(dec)
}

func readAllBody(r *http.Request) ([]byte, error) {
	if r == nil || r.Body == nil {
		return nil, nil
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, domainerr.New(domainerr.PayloadTooLarge, "request body exceeds bodyLimit")
		}
		return nil, domainerr.New(domainerr.ValidationFailed, err.Error())
	}
	return b, nil
}
