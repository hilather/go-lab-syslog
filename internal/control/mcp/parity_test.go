package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/buildinfo"
	"github.com/hilather/go-lab-syslog/internal/capabilities"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

func TestParityEveryRequiredRowHasToolAndREST(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	live := map[string]bool{}
	for tool, err := range cs.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		live[tool.Name] = true
		if strings.HasPrefix(tool.Name, "lab") {
			t.Errorf("rejected prefix on %s", tool.Name)
		}
		if !strings.HasPrefix(tool.Name, "syslog_") {
			t.Errorf("%s must use syslog_ prefix", tool.Name)
		}
	}
	for _, row := range capabilities.Table() {
		parity := false
		restOnly := false
		for _, f := range row.Flags {
			if f == capabilities.ParityRequired {
				parity = true
			}
			if f == capabilities.RESTOnlyProtocol {
				restOnly = true
			}
		}
		if restOnly && row.MCPTool != "" {
			t.Errorf("%s REST_ONLY has tool %s", row.ID, row.MCPTool)
		}
		if restOnly && row.MCPTool != "" && live[row.MCPTool] {
			t.Errorf("%s REST_ONLY tool %s is registered", row.ID, row.MCPTool)
		}
		if !parity {
			continue
		}
		if row.RESTPath == "" || row.RESTMethod == "" {
			t.Errorf("%s missing REST path", row.ID)
		}
		if row.MCPTool == "" {
			t.Errorf("%s missing MCP tool", row.ID)
			continue
		}
		if !live[row.MCPTool] {
			t.Errorf("%s tool %s not registered", row.ID, row.MCPTool)
		}
	}
}

func TestParityGoldens(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "testdata", "mcp", "goldens")
	compareLines(t, filepath.Join(dir, "tools.txt"), capabilities.Tools())
	compareLines(t, filepath.Join(dir, "resources.txt"), capabilities.Resources())
	compareLines(t, filepath.Join(dir, "mutating-tools.txt"), capabilities.MutatingTools())
}

func TestParityLiveResources(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	live := map[string]bool{}
	for res, err := range cs.Resources(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		live[res.URI] = true
	}
	for tmpl, err := range cs.ResourceTemplates(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		live[tmpl.URITemplate] = true
	}
	for _, uri := range capabilities.Resources() {
		if !live[uri] {
			t.Errorf("resource %s not registered", uri)
		}
	}
}

func TestProtocolVersionInBuildinfoAndVersionTool(t *testing.T) {
	info := buildinfo.Current()
	if info.Protocols.MCP != ProtocolVersion {
		t.Fatalf("buildinfo MCP %q want %q", info.Protocols.MCP, ProtocolVersion)
	}
	s, _ := newTestServer(t)
	res := callTool(t, connectClient(t, startHTTP(t, s)), "syslog_version_get", map[string]any{})
	if res.IsError {
		t.Fatalf("version error %+v", res.StructuredContent)
	}
	m, _ := res.StructuredContent.(map[string]any)
	protos, _ := m["protocols"].(map[string]any)
	if protos["mcp"] != ProtocolVersion {
		t.Fatalf("version protocols.mcp %v", protos["mcp"])
	}
}

func TestWaitTimeoutCode(t *testing.T) {
	s, _ := newTestServer(t)
	cs := connectClient(t, startHTTP(t, s))
	res := callTool(t, cs, "syslog_messages_wait", map[string]any{"timeout": "30ms"})
	if !res.IsError {
		t.Fatalf("expected error, got %+v", res.StructuredContent)
	}
	code := domainCode(t, res)
	if code != string(domainerr.WaitTimeout) {
		t.Fatalf("code %q content=%+v structured=%+v", code, res.Content, res.StructuredContent)
	}
}

func TestWaitWipeCode(t *testing.T) {
	s, svc := newTestServer(t)
	cs := connectClient(t, startHTTP(t, s))
	done := make(chan string, 1)
	go func() {
		res := callTool(t, cs, "syslog_messages_wait", map[string]any{
			"timeout": "2s",
			"filter":  map[string]any{"messageContains": "never-match-wipe"},
		})
		done <- domainCode(t, res)
	}()
	waitersAtLeast(t, svc, 1)
	clear := callTool(t, cs, "syslog_messages_clear", map[string]any{})
	if clear.IsError {
		t.Fatalf("clear %+v", clear.StructuredContent)
	}
	select {
	case code := <-done:
		if code != string(domainerr.StoreWiped) {
			t.Fatalf("code %s want store_wiped", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return")
	}
}

func waitersAtLeast(t *testing.T, svc *app.Service, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if svc.Messages().Stats().Waiters >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("wait did not park")
}

func TestNoHealthMetricsSessionTools(t *testing.T) {
	s, _ := newTestServer(t)
	cs := connectClient(t, startHTTP(t, s))
	live := map[string]bool{}
	for tool, err := range cs.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		live[tool.Name] = true
	}
	for _, name := range []string{
		"syslog_health_live", "syslog_health_ready", "syslog_metrics_get",
		"syslog_session_create", "syslog_session_get", "syslog_session_delete",
		"syslog_events_stream",
	} {
		if live[name] {
			t.Errorf("REST_ONLY tool %s must not be registered", name)
		}
	}
}

func TestMessageGetParity(t *testing.T) {
	s, svc := newTestServer(t)
	cs := connectClient(t, startHTTP(t, s))
	if _, err := svc.Messages().Insert(model.Message{
		Transport: "udp",
		Parsed:    model.Parsed{Message: "hello-mcp"},
	}); err != nil {
		t.Fatal(err)
	}
	listed := callTool(t, cs, "syslog_messages_list", map[string]any{"limit": 1})
	if listed.IsError {
		t.Fatalf("list %+v", listed.StructuredContent)
	}
	lm, _ := listed.StructuredContent.(map[string]any)
	items, _ := lm["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items %v", items)
	}
	item, _ := items[0].(map[string]any)
	id, _ := item["id"].(string)
	res := callTool(t, cs, "syslog_message_get", map[string]any{"id": id})
	if res.IsError {
		t.Fatalf("%+v", res.StructuredContent)
	}
	m, _ := res.StructuredContent.(map[string]any)
	if m["id"] != id {
		t.Fatalf("id %v", m["id"])
	}
}

func TestStatelessTrue(t *testing.T) {
	s, _ := newTestServer(t)
	if s.http == nil {
		t.Fatal("http handler")
	}
}

func compareLines(t *testing.T, path string, got []string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (create the golden)", path, err)
	}
	want := strings.Split(strings.TrimSpace(string(body)), "\n")
	if strings.TrimSpace(string(body)) == "" {
		want = nil
	}
	if len(want) != len(got) {
		t.Fatalf("%s: got %d lines want %d\ngot=%v\nwant=%v", path, len(got), len(want), got, want)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("%s:%d got %q want %q", path, i+1, got[i], want[i])
		}
	}
}
