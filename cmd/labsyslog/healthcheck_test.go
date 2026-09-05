package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthcheckReadyOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}))
	t.Cleanup(ts.Close)
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "healthcheck", "--url", ts.URL + "/v1/health/ready"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
}

func TestHealthcheckNotReady(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"not_ready"}`))
	}))
	t.Cleanup(ts.Close)
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "healthcheck", "--url", ts.URL + "/v1/health/ready"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d want 1 stderr=%q", code, stderr.String())
	}
}

func TestHealthcheckUnreachable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labsyslog", "healthcheck", "--url", "http://127.0.0.1:1/v1/health/ready"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d want 1 stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "healthcheck") {
		t.Fatalf("stderr %q", stderr.String())
	}
}
