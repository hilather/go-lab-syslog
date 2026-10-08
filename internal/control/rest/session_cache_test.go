package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/auth"
)

// Session responses carry the CSRF token. They must not be cacheable.
func TestSessionCSRFResponsesAreUncacheable(t *testing.T) {
	ts, _ := newREST(t, "")
	login, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	setAuth(login)
	loginResp, err := ts.Client().Do(login)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("login status %d body=%s", loginResp.StatusCode, b)
	}
	var sess struct {
		CSRF string `json:"csrf"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&sess); err != nil {
		t.Fatal(err)
	}
	cookie := sessionCookie(loginResp)
	if cookie == "" || sess.CSRF == "" {
		t.Fatal("missing cookie or csrf")
	}
	assertNoStore(t, "POST /v1/session", loginResp)

	getReq, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	getReq.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	getResp, err := ts.Client().Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	body, _ := io.ReadAll(getResp.Body)
	if getResp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"csrf"`) {
		t.Fatalf("GET /v1/session status %d body=%s", getResp.StatusCode, body)
	}
	assertNoStore(t, "GET /v1/session", getResp)

	del, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	del.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	del.Header.Set(auth.CSRFHeader, sess.CSRF)
	delResp, err := ts.Client().Do(del)
	if err != nil {
		t.Fatal(err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(delResp.Body)
		t.Fatalf("DELETE /v1/session status %d body=%s", delResp.StatusCode, b)
	}
	assertNoStore(t, "DELETE /v1/session", delResp)
}

func assertNoStore(t *testing.T, what string, resp *http.Response) {
	t.Helper()
	if !strings.Contains(strings.ToLower(resp.Header.Get("Cache-Control")), "no-store") {
		t.Errorf("%s Cache-Control=%q, want no-store", what, resp.Header.Get("Cache-Control"))
	}
}
