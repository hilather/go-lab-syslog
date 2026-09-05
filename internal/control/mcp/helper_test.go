package mcp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/testutil"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var testBearerSecret = strings.Repeat("t", auth.MinTokenBytes)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
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
	if err := svc.Start(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func newTestServer(t *testing.T) (*Server, *app.Service) {
	t.Helper()
	return newTestServerSpec(t, `
  management:
    mcp:
      allowLegacyClients: true
`)
}

func newTestServerSpec(t *testing.T, extraSpec string) (*Server, *app.Service) {
	t.Helper()
	svc := newService(t, extraSpec)
	s, err := New(Config{Service: svc, AllowLegacyClients: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, svc
}

type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (b bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set(headerAuthorization, "Bearer "+b.token)
	base := b.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

func startHTTP(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func connectClient(t *testing.T, ts *httptest.Server) *sdk.ClientSession {
	t.Helper()
	return connectClientAuth(t, ts, testBearerSecret)
}

func newReaderServer(t *testing.T) (*Server, *app.Service) {
	t.Helper()
	dir := t.TempDir()
	adminTok := filepath.Join(dir, "token")
	readerTok := filepath.Join(dir, "reader.token")
	if err := os.WriteFile(adminTok, []byte(testBearerSecret), 0o644); err != nil {
		t.Fatal(err)
	}
	readerSecret := strings.Repeat("r", auth.MinTokenBytes)
	if err := os.WriteFile(readerTok, []byte(readerSecret), 0o644); err != nil {
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
        secretFile: ` + adminTok + `
      - id: reader
        role: reader
        secretFile: ` + readerTok + `
  admission:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
`
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
	if err := svc.Start(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	s, err := New(Config{Service: svc, AllowLegacyClients: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, svc
}

func connectClientAuth(t *testing.T, ts *httptest.Server, token string) *sdk.ClientSession {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "labsyslog-test", Version: "dev"}, nil)
	session, err := client.Connect(t.Context(), &sdk.StreamableClientTransport{
		Endpoint:             ts.URL,
		DisableStandaloneSSE: true,
		HTTPClient:           &http.Client{Transport: bearerRoundTripper{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callTool(t *testing.T, cs *sdk.ClientSession, name string, args any) *sdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

func domainCode(t *testing.T, res *sdk.CallToolResult) string {
	t.Helper()
	if res == nil || !res.IsError {
		t.Fatalf("expected tool error, got %+v", res)
	}
	if m, ok := res.StructuredContent.(map[string]any); ok {
		if code, _ := m["code"].(string); code != "" {
			return code
		}
	}
	for _, c := range res.Content {
		tc, ok := c.(*sdk.TextContent)
		if !ok || tc.Text == "" {
			continue
		}
		if i := strings.Index(tc.Text, ":"); i > 0 {
			return strings.TrimSpace(tc.Text[:i])
		}
		return tc.Text
	}
	t.Fatalf("no domain code in %+v", res)
	return ""
}
