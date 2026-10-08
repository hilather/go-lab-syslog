package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// openStdioSession authenticates the admin token once, the way
// `labsyslog mcp-stdio` does, and keeps that principal on the server.
// rewrite replaces the auth.tokens block and is meant to be followed by Reset.
func openStdioSession(t *testing.T) (svc *app.Service, session *sdk.ClientSession, adminTok string, rewrite func(tokens string)) {
	t.Helper()
	dir := t.TempDir()
	adminTok = filepath.Join(dir, "admin.token")
	if err := os.WriteFile(adminTok, []byte(testBearerSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	rewrite = func(tokens string) {
		t.Helper()
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
` + tokens + `
  admission:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
  management:
    mcp:
      allowLegacyClients: true
`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rewrite("      - id: operator\n        role: administrator\n        secretFile: " + adminTok + "\n")

	var err error
	svc, err = app.New(app.Config{
		BootstrapPath: path,
		Compiler: compiler.Options{
			ConfigDir:        dir,
			ManagementListen: "off",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := testutil.Context(t)
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	principal, err := svc.Verifier().AuthenticateBearer(testBearerSecret)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{
		Service:            svc,
		AllowLegacyClients: true,
		FixedPrincipal:     &principal,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	go func() {
		_ = s.sdk.Run(ctx, serverTransport)
	}()
	client := sdk.NewClient(&sdk.Implementation{Name: "stdio-repro", Version: "dev"}, nil)
	session, err = client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return svc, session, adminTok, rewrite
}

// Removing the startup token id on reset must drop the captured
// administrator grant before another mutation or read is accepted.
func TestStdioPrincipalLosesAdminAfterTokenRotation(t *testing.T) {
	svc, session, _, rewrite := openStdioSession(t)
	dir := t.TempDir()
	readerTok := filepath.Join(dir, "reader.token")
	readerSecret := strings.Repeat("r", auth.MinTokenBytes)
	if err := os.WriteFile(readerTok, []byte(readerSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	rewrite("      - id: reader\n        role: reader\n        secretFile: " + readerTok + "\n")

	ctx := testutil.Context(t)
	if err := svc.Reset(ctx, "operator", "rotate token"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Verifier().AuthenticateBearer(testBearerSecret); err == nil {
		t.Fatal("rotated admin secret still authenticates")
	}
	if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Parsed: model.Parsed{Message: "kept"}}); err != nil {
		t.Fatal(err)
	}

	res := callTool(t, session, "syslog_messages_clear", map[string]any{"reason": "after rotation"})
	if !res.IsError {
		t.Fatalf("syslog_messages_clear succeeded after the admin token was removed; store messages=%d", svc.Messages().Stats().Messages)
	}
	if code := domainCode(t, res); code != "unauthorized" {
		t.Fatalf("domainCode = %s, want unauthorized", code)
	}
	if got := svc.Messages().Stats().Messages; got != 1 {
		t.Fatalf("stdio clear changed the store to %d messages after the admin token was removed", got)
	}

	_, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: "labsyslog://status"})
	if err == nil {
		t.Fatal("labsyslog://status read succeeded after the admin token was removed")
	}
	var rpc *jsonrpc.Error
	if !errors.As(err, &rpc) || !strings.Contains(string(rpc.Data), `"code":"unauthorized"`) {
		t.Fatalf("labsyslog://status read error = %v, want unauthorized", err)
	}
}

// Demoting the same token id must use the live role. Clear is forbidden;
// a syslog.read tool still succeeds. The store is not wiped.
func TestStdioPrincipalUsesLiveScopesAfterDemotion(t *testing.T) {
	svc, session, adminTok, rewrite := openStdioSession(t)
	rewrite("      - id: operator\n        role: reader\n        secretFile: " + adminTok + "\n")

	ctx := testutil.Context(t)
	if err := svc.Reset(ctx, "operator", "demote token"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Parsed: model.Parsed{Message: "kept"}}); err != nil {
		t.Fatal(err)
	}

	res := callTool(t, session, "syslog_messages_clear", map[string]any{"reason": "after demotion"})
	if !res.IsError {
		t.Fatalf("syslog_messages_clear succeeded under the cached administrator scopes; store messages=%d", svc.Messages().Stats().Messages)
	}
	if code := domainCode(t, res); code != "forbidden" {
		t.Fatalf("domainCode = %s, want forbidden", code)
	}
	if got := svc.Messages().Stats().Messages; got != 1 {
		t.Fatalf("stdio clear changed the store to %d messages after demotion", got)
	}

	status := callTool(t, session, "syslog_status_get", map[string]any{})
	if status.IsError {
		t.Fatalf("syslog_status_get failed for the live reader: %v", status)
	}
}
