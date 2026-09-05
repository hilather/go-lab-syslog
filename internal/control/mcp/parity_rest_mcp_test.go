package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/control/rest"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const missingID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestParityRESTAndMCPSameService(t *testing.T) {
	svc := newService(t, `
  management:
    mcp:
      allowLegacyClients: true
`)
	mcpSrv, err := New(Config{Service: svc, AllowLegacyClients: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mcpSrv.Close)
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpSrv.Handler())
	mux.Handle("/", rest.New(svc))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	cs := connectClientPath(t, ts, "/mcp", testBearerSecret)

	if _, err := svc.Messages().Insert(model.Message{
		Transport: "udp",
		Raw:       []byte("raw-parity"),
		Parsed:    model.Parsed{Message: "parity-probe"},
	}); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	var deferred []capabilities.Row
	for _, row := range capabilities.Table() {
		parity := false
		for _, f := range row.Flags {
			if f == capabilities.ParityRequired {
				parity = true
			}
		}
		if !parity {
			continue
		}
		seen[row.ID] = true
		if row.ID == "state.reset" || row.ID == "messages.clear" {
			deferred = append(deferred, row)
			continue
		}
		t.Run(row.ID, func(t *testing.T) {
			invokeParityRow(t, ts, cs, svc, row)
		})
	}
	for _, row := range deferred {
		t.Run(row.ID, func(t *testing.T) {
			invokeParityRow(t, ts, cs, svc, row)
		})
	}
	for _, row := range capabilities.Table() {
		for _, f := range row.Flags {
			if f == capabilities.ParityRequired && !seen[row.ID] {
				t.Errorf("no invoker for %s", row.ID)
			}
		}
	}
}

func connectClientPath(t *testing.T, ts *httptest.Server, path, token string) *sdk.ClientSession {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "labsyslog-test", Version: "dev"}, nil)
	session, err := client.Connect(t.Context(), &sdk.StreamableClientTransport{
		Endpoint:             ts.URL + path,
		DisableStandaloneSSE: true,
		HTTPClient:           &http.Client{Transport: bearerRoundTripper{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func invokeParityRow(t *testing.T, ts *httptest.Server, cs *sdk.ClientSession, svc *app.Service, row capabilities.Row) {
	t.Helper()
	rev := svc.State(t.Context()).Revision
	switch row.ID {
	case "version.get", "capabilities.get", "status.get", "schema.config.get", "features.list",
		"state.get", "stats.get", "audit.query":
		restOK(t, ts, http.MethodGet, row.RESTPath, nil)
		mcpOK(t, cs, row.MCPTool, map[string]any{})
	case "state.validate":
		doc := map[string]any{
			"apiVersion": "labsyslog.dev/v1alpha1",
			"kind":       "LabSyslog",
			"metadata":   map[string]any{"name": "lab-sink"},
			"spec":       map[string]any{},
		}
		body, _ := json.Marshal(doc)
		restOK(t, ts, http.MethodPost, row.RESTPath, body)
		mcpOK(t, cs, row.MCPTool, map[string]any{"document": doc})
	case "state.export":
		restOK(t, ts, http.MethodGet, row.RESTPath, nil)
		mcpOK(t, cs, row.MCPTool, map[string]any{"format": "yaml"})
	case "change.plan":
		payload := map[string]any{"expectedRevision": rev, "operations": []any{}}
		body, _ := json.Marshal(payload)
		restOK(t, ts, http.MethodPost, row.RESTPath, body)
		mcpOK(t, cs, row.MCPTool, payload)
	case "change.apply":
		storeOp := map[string]any{
			"type": app.OpReplaceStoreCaps,
			"store": map[string]any{
				"maxMessages": 10000,
				"maxBytes":    "256MiB",
				"fullPolicy":  "evict_oldest",
				"maxWait":     "60s",
				"rawRetain":   true,
			},
		}
		restPayload := map[string]any{
			"expectedRevision": rev,
			"idempotencyKey":   "parity-apply-rest",
			"operations":       []any{storeOp},
		}
		body, _ := json.Marshal(restPayload)
		restOK(t, ts, http.MethodPost, row.RESTPath, body)
		mcpPayload := map[string]any{
			"expectedRevision": svc.State(t.Context()).Revision,
			"idempotencyKey":   "parity-apply-mcp",
			"operations":       []any{storeOp},
		}
		mcpOK(t, cs, row.MCPTool, mcpPayload)
	case "messages.list":
		restOK(t, ts, http.MethodGet, row.RESTPath+"?limit=10", nil)
		mcpOK(t, cs, row.MCPTool, map[string]any{"limit": 10})
	case "message.get":
		assertSameCode(t, restCode(t, ts, http.MethodGet, "/v1/messages/"+missingID, nil), mcpToolCode(t, cs, row.MCPTool, map[string]any{"id": missingID}))
		id := newestMessageID(t, svc)
		restOK(t, ts, http.MethodGet, "/v1/messages/"+id, nil)
		mcpOK(t, cs, row.MCPTool, map[string]any{"id": id})
	case "message.raw.get":
		assertSameCode(t, restCode(t, ts, http.MethodGet, "/v1/messages/"+missingID+"/raw", nil), mcpToolCode(t, cs, row.MCPTool, map[string]any{"id": missingID}))
		id := newestMessageID(t, svc)
		restOK(t, ts, http.MethodGet, "/v1/messages/"+id+"/raw", nil)
		mcpOK(t, cs, row.MCPTool, map[string]any{"id": id})
	case "audit.get":
		assertSameCode(t, restCode(t, ts, http.MethodGet, "/v1/audit/"+missingID, nil), mcpToolCode(t, cs, row.MCPTool, map[string]any{"id": missingID}))
		items := svc.AuditRing().List()
		if len(items) == 0 {
			t.Fatal("expected audit after apply/plan")
		}
		id := items[0].ID
		restOK(t, ts, http.MethodGet, "/v1/audit/"+id, nil)
		mcpOK(t, cs, row.MCPTool, map[string]any{"id": id})
	case "messages.wait":
		payload := map[string]any{"timeout": "20ms", "filter": map[string]any{"messageContains": "never-wait-parity"}}
		body, _ := json.Marshal(payload)
		assertSameCode(t, restCode(t, ts, http.MethodPost, row.RESTPath, body), mcpToolCode(t, cs, row.MCPTool, payload))
	case "message.delete":
		assertSameCode(t, restCode(t, ts, http.MethodDelete, "/v1/messages/"+missingID, nil), mcpToolCode(t, cs, row.MCPTool, map[string]any{"id": missingID}))
		if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Parsed: model.Parsed{Message: "to-delete"}}); err != nil {
			t.Fatal(err)
		}
		id := newestMessageID(t, svc)
		restOK(t, ts, http.MethodDelete, "/v1/messages/"+id, nil)
		if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Parsed: model.Parsed{Message: "to-delete-mcp"}}); err != nil {
			t.Fatal(err)
		}
		id = newestMessageID(t, svc)
		mcpOK(t, cs, row.MCPTool, map[string]any{"id": id})
	case "messages.clear":
		restOK(t, ts, http.MethodPost, row.RESTPath, nil)
		mcpOK(t, cs, row.MCPTool, map[string]any{})
	case "state.reset":
		restOK(t, ts, http.MethodPost, row.RESTPath, nil)
		mcpOK(t, cs, row.MCPTool, map[string]any{})
	default:
		t.Fatalf("no parity invoker for %s", row.ID)
	}
}

func restOK(t *testing.T, ts *httptest.Server, method, path string, body []byte) {
	t.Helper()
	status, code, _ := restCall(t, ts, method, path, body)
	if status >= 400 {
		t.Fatalf("REST %s %s status %d code %s", method, path, status, code)
	}
}

func mcpOK(t *testing.T, cs *sdk.ClientSession, name string, args any) {
	t.Helper()
	res := callTool(t, cs, name, args)
	if res.IsError {
		t.Fatalf("MCP %s %+v content=%+v", name, res.StructuredContent, res.Content)
	}
}

func restCode(t *testing.T, ts *httptest.Server, method, path string, body []byte) string {
	t.Helper()
	_, code, _ := restCall(t, ts, method, path, body)
	return code
}

func mcpToolCode(t *testing.T, cs *sdk.ClientSession, name string, args any) string {
	t.Helper()
	return domainCode(t, callTool(t, cs, name, args))
}

func restCall(t *testing.T, ts *httptest.Server, method, path string, body []byte) (int, string, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testBearerSecret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	code := ""
	if strings.Contains(resp.Header.Get("Content-Type"), "problem+json") {
		var p domainerr.Problem
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
		code = string(p.Code)
	}
	return resp.StatusCode, code, b
}

func assertSameCode(t *testing.T, rest, mcp string) {
	t.Helper()
	if rest == "" || mcp == "" {
		t.Fatalf("empty code rest=%q mcp=%q", rest, mcp)
	}
	if rest != mcp {
		t.Fatalf("domain code rest=%q mcp=%q", rest, mcp)
	}
}

func newestMessageID(t *testing.T, svc *app.Service) string {
	t.Helper()
	out, err := svc.Messages().List(store.ListFilter{}, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) == 0 {
		t.Fatal("no messages")
	}
	return out.Items[0].ID
}
