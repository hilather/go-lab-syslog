package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/observability"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

var testBearerSecret = strings.Repeat("t", auth.MinTokenBytes)

func TestUnknownRouteProblemJSON(t *testing.T) {
	ts, _ := newREST(t, "")
	resp := get(t, ts, "/v1/does-not-exist")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
	p := decodeProblem(t, resp)
	if p.Code != domainerr.NotFound {
		t.Fatalf("code %s", p.Code)
	}
	if p.Type != "https://labsyslog.dev/errors/not_found" {
		t.Fatalf("type %s", p.Type)
	}
	if p.Detail != "no such route" {
		t.Fatalf("detail %q", p.Detail)
	}
	if ct := resp.Header.Get("Content-Type"); ct != problemType {
		t.Fatalf("content-type %s", ct)
	}
}

func TestMissingMessageKeepsDetail(t *testing.T) {
	ts, _ := newREST(t, "")
	resp := get(t, ts, "/v1/messages/01ARZ3NDEKTSV4RRFFQ69G5FAV")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
	p := decodeProblem(t, resp)
	if p.Code != domainerr.NotFound {
		t.Fatalf("code %s", p.Code)
	}
	if p.Detail != "message not found" {
		t.Fatalf("detail %q", p.Detail)
	}
}

func TestContractPerCapability(t *testing.T) {
	ts, svc := newREST(t, "")
	rev := svc.State(testutil.Context(t)).Revision
	for _, row := range capabilities.Table() {
		t.Run(row.ID, func(t *testing.T) {
			path := row.RESTPath
			path = strings.ReplaceAll(path, "{id}", "01ARZ3NDEKTSV4RRFFQ69G5FAV")
			var body io.Reader
			req, err := http.NewRequest(row.RESTMethod, ts.URL+path, nil)
			if err != nil {
				t.Fatal(err)
			}
			setAuth(req)
			switch row.ID {
			case "ui.static":
				resp, err := ts.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusNotFound {
					t.Fatalf("ui.static status %d want 404 until UI-001", resp.StatusCode)
				}
				return
			case "metrics.scrape":
				resp, err := ts.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusNotFound {
					t.Fatalf("metrics default status %d want 404", resp.StatusCode)
				}
				return
			case "state.validate":
				body = strings.NewReader(`{"apiVersion":"labsyslog.dev/v1alpha1","kind":"LabSyslog","metadata":{"name":"lab-sink"},"spec":{}}`)
				req, _ = http.NewRequest(row.RESTMethod, ts.URL+path, body)
				req.Header.Set("Content-Type", "application/json")
				setAuth(req)
			case "change.plan":
				payload, _ := json.Marshal(map[string]any{
					"expectedRevision": rev,
					"operations":       []any{},
				})
				req, _ = http.NewRequest(row.RESTMethod, ts.URL+path, bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				setAuth(req)
			case "change.apply":
				payload, _ := json.Marshal(map[string]any{
					"expectedRevision": rev,
					"operations":       []any{},
					"idempotencyKey":   "contract-apply",
				})
				req, _ = http.NewRequest(row.RESTMethod, ts.URL+path, bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", "contract-apply")
				setAuth(req)
			case "messages.wait":
				payload, _ := json.Marshal(map[string]any{"timeout": "20ms"})
				req, _ = http.NewRequest(row.RESTMethod, ts.URL+path, bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				setAuth(req)
			case "events.stream":
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				req, _ = http.NewRequestWithContext(ctx, row.RESTMethod, ts.URL+path, nil)
				setAuth(req)
				resp, err := ts.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode != http.StatusOK {
					resp.Body.Close()
					t.Fatalf("events.stream status %d", resp.StatusCode)
				}
				if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
					resp.Body.Close()
					t.Fatalf("events.stream content-type %s", ct)
				}
				cancel()
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				return
			}
			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if row.ID == "messages.wait" {
				if resp.StatusCode != http.StatusGatewayTimeout {
					t.Fatalf("wait status %d want 504", resp.StatusCode)
				}
				p := decodeProblem(t, resp)
				if p.Code != domainerr.WaitTimeout {
					t.Fatalf("wait code %s", p.Code)
				}
				return
			}
			if row.ID == "message.get" || row.ID == "message.raw.get" || row.ID == "message.delete" || row.ID == "audit.get" {
				if resp.StatusCode != http.StatusNotFound {
					t.Fatalf("%s status %d want 404", row.ID, resp.StatusCode)
				}
				return
			}
			if resp.StatusCode == http.StatusNotFound {
				t.Fatalf("%s %s %s was 404", row.ID, row.RESTMethod, path)
			}
		})
	}
}

func TestHealthLiveWithoutReady(t *testing.T) {
	svc := newService(t, "")
	s := newServer(svc)
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	live := get(t, ts, "/v1/health/live")
	defer live.Body.Close()
	if live.StatusCode != 200 {
		t.Fatalf("live %d want 200 while process is running", live.StatusCode)
	}
	ready := get(t, ts, "/v1/health/ready")
	defer ready.Body.Close()
	if ready.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("ready %d want 503 before bind", ready.StatusCode)
	}
}

func TestHealthAndFeatures(t *testing.T) {
	ts, _ := newREST(t, "")
	live := get(t, ts, "/v1/health/live")
	defer live.Body.Close()
	if live.StatusCode != 200 {
		t.Fatalf("live %d", live.StatusCode)
	}
	ready := get(t, ts, "/v1/health/ready")
	defer ready.Body.Close()
	if ready.StatusCode != 200 {
		t.Fatalf("ready %d", ready.StatusCode)
	}
	feat := get(t, ts, "/v1/features")
	defer feat.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(feat.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["tls"] != false {
		t.Fatalf("tls %v", body["tls"])
	}
	framing, _ := body["framing"].([]any)
	if len(framing) != 3 {
		t.Fatalf("framing %v", body["framing"])
	}
}

func TestStateDriftedAndExport(t *testing.T) {
	ts, svc := newREST(t, "")
	st := get(t, ts, "/v1/state")
	defer st.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(st.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["drifted"] != false {
		t.Fatal("fresh state should not be drifted")
	}
	if body["revision"] != svc.State(testutil.Context(t)).Revision {
		t.Fatal("revision mismatch")
	}
	exp := get(t, ts, "/v1/state:export")
	defer exp.Body.Close()
	if exp.StatusCode != 200 {
		t.Fatalf("export %d", exp.StatusCode)
	}
	if ct := exp.Header.Get("Content-Type"); ct != "application/yaml" {
		t.Fatalf("export content-type %s", ct)
	}
}

func TestSchemaConfig(t *testing.T) {
	ts, _ := newREST(t, "")
	resp := get(t, ts, "/v1/schema/config")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	if raw["title"] != "LabSyslog" {
		t.Fatalf("title %v", raw["title"])
	}
}

func TestMessagesListCursorAndRaw(t *testing.T) {
	ts, svc := newREST(t, "")
	for i := 0; i < 3; i++ {
		if _, err := svc.Messages().Insert(model.Message{
			Transport: "udp",
			Raw:       []byte("raw-body"),
			Parsed:    model.Parsed{Message: "m"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	resp := get(t, ts, "/v1/messages?limit=2")
	defer resp.Body.Close()
	var list struct {
		Items      []map[string]any `json:"items"`
		NextCursor string           `json:"nextCursor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("items %d", len(list.Items))
	}
	if _, ok := list.Items[0]["raw"]; ok {
		t.Fatal("list must omit raw")
	}
	if list.NextCursor == "" {
		t.Fatal("missing nextCursor")
	}
	page2 := get(t, ts, "/v1/messages?limit=2&cursor="+list.NextCursor)
	defer page2.Body.Close()
	if page2.StatusCode != 200 {
		t.Fatalf("page2 %d", page2.StatusCode)
	}
	stale := get(t, ts, "/v1/messages?cursor=not-a-cursor")
	defer stale.Body.Close()
	if stale.StatusCode != 400 {
		t.Fatalf("stale %d", stale.StatusCode)
	}
	p := decodeProblem(t, stale)
	if p.Code != domainerr.CursorStale {
		t.Fatalf("code %s", p.Code)
	}

	id, _ := list.Items[0]["id"].(string)
	raw := get(t, ts, "/v1/messages/"+id+"/raw")
	defer raw.Body.Close()
	b, _ := io.ReadAll(raw.Body)
	if raw.StatusCode != 200 || string(b) != "raw-body" {
		t.Fatalf("raw %d %q", raw.StatusCode, b)
	}
}

func TestMetricsPublicPath(t *testing.T) {
	ts, _ := newREST(t, `
  observability:
    metrics:
      publicPath: true
`)
	resp := get(t, ts, "/v1/metrics")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/openmetrics-text") {
		t.Fatalf("content-type %s", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	samples, err := observability.ParseOpenMetrics(string(body))
	if err != nil {
		t.Fatal(err)
	}
	by := observability.ByName(samples)
	for _, name := range observability.RequiredNames() {
		if _, ok := by[name]; !ok {
			t.Errorf("missing series %s", name)
		}
	}
}

func TestMetricsPublicPathTrueUnauthenticated(t *testing.T) {
	ts, _ := newREST(t, `
  observability:
    metrics:
      publicPath: true
`)
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unauthenticated scrape status %d", resp.StatusCode)
	}
}

func TestMetricsPublicPathFalseEvenWithAuth(t *testing.T) {
	ts, _ := newREST(t, "")
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 32))
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d want 404 even with auth", resp.StatusCode)
	}
	p := decodeProblem(t, resp)
	if p.Detail != "metrics publicPath is false" {
		t.Fatalf("detail %q", p.Detail)
	}
}

func TestMetricsDisabledNotUnknownRoute(t *testing.T) {
	ts, _ := newREST(t, "")
	resp := get(t, ts, "/v1/metrics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
	p := decodeProblem(t, resp)
	if p.Code != domainerr.NotFound {
		t.Fatalf("code %s", p.Code)
	}
	if p.Detail == "no such route" {
		t.Fatal("metrics publicPath false must not look like an unknown route")
	}
	if p.Detail != "metrics publicPath is false" {
		t.Fatalf("detail %q", p.Detail)
	}
}

func newREST(t *testing.T, extraSpec string) (*httptest.Server, *app.Service) {
	t.Helper()
	svc := newService(t, extraSpec)
	if err := svc.Start(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
	s := newServer(svc)
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return ts, svc
}

func newService(t *testing.T, extraSpec string) *app.Service {
	t.Helper()
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerSecret), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec:
  listeners:
    udp:
      enabled: true
      address: "127.0.0.1:0"
    tcp:
      enabled: true
      address: "127.0.0.1:0"
  auth:
    tokens:
      - id: operator
        role: administrator
        secretFile: ` + tok + `
  admission:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
` + extraSpec
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.New(app.Config{
		BootstrapPath: path,
		Compiler: compiler.Options{
			ConfigDir:        dir,
			ManagementListen: "off",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func newTestServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func get(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	setAuth(req)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+testBearerSecret)
}

func decodeProblem(t *testing.T, resp *http.Response) domainerr.Problem {
	t.Helper()
	var p domainerr.Problem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	return p
}
