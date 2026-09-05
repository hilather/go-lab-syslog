package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

func TestMissingBearerOnStateIs401(t *testing.T) {
	ts, _ := newREST(t, "")
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d want 401 body=%s", resp.StatusCode, body)
	}
	p := decodeProblem(t, resp)
	if p.Code != domainerr.Unauthorized {
		t.Fatalf("code %s", p.Code)
	}
	if !strings.Contains(resp.Header.Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("WWW-Authenticate %q", resp.Header.Get("WWW-Authenticate"))
	}
}

func TestHealthUnauthenticated(t *testing.T) {
	ts, _ := newREST(t, "")
	for _, path := range []string{"/v1/health/live", "/v1/health/ready"} {
		req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("%s status %d body=%s", path, resp.StatusCode, body)
		}
		resp.Body.Close()
	}
}

func TestCSRFMissingOnCookiePostIs403(t *testing.T) {
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
		body, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("session %d body=%s", loginResp.StatusCode, body)
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

	clear, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/messages:clear", nil)
	if err != nil {
		t.Fatal(err)
	}
	clear.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	resp, err := ts.Client().Do(clear)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d want 403 body=%s", resp.StatusCode, body)
	}
	p := decodeProblem(t, resp)
	if p.Code != domainerr.Forbidden {
		t.Fatalf("code %s", p.Code)
	}

	okReq, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/messages:clear", nil)
	if err != nil {
		t.Fatal(err)
	}
	okReq.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	okReq.Header.Set(auth.CSRFHeader, sess.CSRF)
	okResp, err := ts.Client().Do(okReq)
	if err != nil {
		t.Fatal(err)
	}
	defer okResp.Body.Close()
	if okResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(okResp.Body)
		t.Fatalf("csrf post %d body=%s", okResp.StatusCode, body)
	}
}

func TestOriginNotAllowed(t *testing.T) {
	ts, _ := newREST(t, "")
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	setAuth(req)
	req.Header.Set("Origin", "https://evil.example")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d want 403 body=%s", resp.StatusCode, body)
	}
	p := decodeProblem(t, resp)
	if p.Code != domainerr.OriginNotAllowed {
		t.Fatalf("code %s", p.Code)
	}
}

func TestOriginExactAllowlist(t *testing.T) {
	ts, _ := newREST(t, `
  management:
    allowedOrigins: ["https://ui.lab"]
`)
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	setAuth(req)
	req.Header.Set("Origin", "https://ui.lab")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
}

func TestAuditApplyThenResetWipesRing(t *testing.T) {
	ts, svc := newREST(t, "")
	rev := svc.State(testutil.Context(t)).Revision
	apply := postJSON(t, ts, "/v1/changes:apply", map[string]any{
		"expectedRevision": rev,
		"operations":       []any{map[string]any{"type": "replaceObservability", "logLevel": "debug"}},
		"idempotencyKey":   "audit-apply",
	})
	defer apply.Body.Close()
	if apply.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(apply.Body)
		t.Fatalf("apply %d body=%s", apply.StatusCode, body)
	}
	listed := get(t, ts, "/v1/audit")
	defer listed.Body.Close()
	var body struct {
		Items []struct {
			Operation string `json:"operation"`
			Actor     string `json:"actor"`
		} `json:"items"`
	}
	if err := json.NewDecoder(listed.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !restAuditHas(body.Items, "apply") {
		t.Fatalf("apply missing: %+v", body.Items)
	}
	if body.Items[0].Actor == "" {
		t.Fatal("audit actor empty")
	}

	reset := postJSON(t, ts, "/v1/state:reset", nil)
	defer reset.Body.Close()
	if reset.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(reset.Body)
		t.Fatalf("reset %d body=%s", reset.StatusCode, b)
	}
	after := get(t, ts, "/v1/audit")
	defer after.Body.Close()
	body.Items = nil
	if err := json.NewDecoder(after.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if restAuditHas(body.Items, "apply") {
		t.Fatalf("apply survived reset: %+v", body.Items)
	}
	if !restAuditHas(body.Items, "reset") {
		t.Fatalf("reset missing: %+v", body.Items)
	}
}

func TestReaderForbiddenOnReset(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerSecret), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.yaml")
	body := `apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec:
  listeners:
    udp:
      address: "127.0.0.1:0"
    tcp:
      address: "127.0.0.1:0"
  auth:
    tokens:
      - id: observer
        role: reader
        secretFile: ` + tok + `
  admission:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
`
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.New(app.Config{
		BootstrapPath: cfg,
		Compiler:      compiler.Options{ConfigDir: dir, ManagementListen: "off"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if err := svc.Start(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
	ts := newTestServer(t, newServer(svc))

	st := get(t, ts, "/v1/state")
	defer st.Body.Close()
	if st.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(st.Body)
		t.Fatalf("reader state %d body=%s", st.StatusCode, b)
	}
	reset := postJSON(t, ts, "/v1/state:reset", nil)
	defer reset.Body.Close()
	if reset.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(reset.Body)
		t.Fatalf("reader reset %d body=%s", reset.StatusCode, b)
	}
	p := decodeProblem(t, reset)
	if p.Code != domainerr.Forbidden {
		t.Fatalf("code %s", p.Code)
	}
}

func TestOPTIONSIs403(t *testing.T) {
	ts, _ := newREST(t, "")
	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/v1/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	setAuth(req)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func sessionCookie(resp *http.Response) string {
	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieName {
			return c.Value
		}
	}
	return ""
}

func restAuditHas(items []struct {
	Operation string `json:"operation"`
	Actor     string `json:"actor"`
}, op string) bool {
	for _, e := range items {
		if e.Operation == op {
			return true
		}
	}
	return false
}
