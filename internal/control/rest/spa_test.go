package rest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSPAFallbackWhenEnabled(t *testing.T) {
	s := newServer(newService(t, ""))
	s.ui = stubSPA()
	s.uiEnabled = func() bool { return true }

	got := doSPA(t, s, http.MethodGet, "/")
	if got.Code != http.StatusOK {
		t.Fatalf("GET / code=%d body=%s", got.Code, got.Body.String())
	}
	if !strings.Contains(got.Body.String(), "LabSyslog") {
		t.Fatalf("GET / body=%s", got.Body.String())
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET / content-type=%q", ct)
	}

	preview := doSPA(t, s, http.MethodGet, "/messages")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "LabSyslog") {
		t.Fatalf("SPA fallback code=%d body=%s", preview.Code, preview.Body.String())
	}
}

func TestSPADisabledIs404(t *testing.T) {
	s := newServer(newService(t, ""))
	s.ui = stubSPA()
	s.uiEnabled = func() bool { return false }

	got := doSPA(t, s, http.MethodGet, "/")
	if got.Code != http.StatusNotFound {
		t.Fatalf("GET / code=%d body=%s", got.Code, got.Body.String())
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("content-type=%q", ct)
	}
	body := got.Body.String()
	if strings.Contains(body, "<!doctype") || strings.Contains(body, "<html") {
		t.Fatalf("disabled UI served HTML: %s", body)
	}
}

func TestSPADoesNotCaptureAPI(t *testing.T) {
	s := newServer(newService(t, ""))
	s.ui = stubSPA()
	s.uiEnabled = func() bool { return true }

	req := httptest.NewRequest(http.MethodGet, "/v1/does-not-exist", nil)
	setAuth(req)
	got := httptest.NewRecorder()
	s.ServeHTTP(got, req)
	if got.Code != http.StatusNotFound {
		t.Fatalf("code=%d", got.Code)
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("content-type=%q", ct)
	}
	if strings.Contains(got.Body.String(), "<!doctype") {
		t.Fatal("API miss served HTML")
	}
}

func stubSPA() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><title>LabSyslog</title><div id=\"root\"></div>")
	})
}

func doSPA(t *testing.T, s *Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	return w
}
