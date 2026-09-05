package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

func TestBearerRequired(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d body=%s", w.Code, w.Body.Bytes())
	}
	if !strings.Contains(w.Header().Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("WWW-Authenticate %q", w.Header().Get("WWW-Authenticate"))
	}
	if rpcDataCode(t, w.Body.Bytes()) != string(domainerr.Unauthorized) {
		t.Fatalf("data %s", w.Body.String())
	}
}

func TestNoBasic(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", w.Code)
	}
}

func TestCookieNotAccepted(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "not-a-session"})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", w.Code)
	}
}

func TestReaderForbiddenOnReset(t *testing.T) {
	s, _ := newReaderServer(t)
	ts := startHTTP(t, s)
	cs := connectClientAuth(t, ts, strings.Repeat("r", auth.MinTokenBytes))
	res := callTool(t, cs, "syslog_state_reset", map[string]any{})
	if domainCode(t, res) != string(domainerr.Forbidden) {
		t.Fatalf("code %s want forbidden", domainCode(t, res))
	}
}

func rpcDataCode(t *testing.T, body []byte) string {
	t.Helper()
	var env struct {
		Error struct {
			Data struct {
				Code string `json:"code"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	return env.Error.Data.Code
}
