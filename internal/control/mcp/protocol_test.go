package mcp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

func TestProtocolHeaderRequiredWhenLegacyOff(t *testing.T) {
	s, _ := newTestServerSpec(t, `
  management:
    mcp:
      allowLegacyClients: false
`)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(headerAuthorization, "Bearer "+testBearerSecret)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d body=%s", w.Code, w.Body.Bytes())
	}
	if rpcDataCode(t, w.Body.Bytes()) != string(domainerr.ValidationFailed) {
		t.Fatalf("body %s", w.Body.String())
	}
}

func TestProtocolHeaderOptionalWhenLegacyOn(t *testing.T) {
	s, _ := newTestServerSpec(t, `
  management:
    mcp:
      allowLegacyClients: true
`)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(headerAuthorization, "Bearer "+testBearerSecret)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "MCP-Protocol-Version") {
		t.Fatalf("legacy clients must not require protocol header; body=%s", w.Body.Bytes())
	}
}

func TestAllowLegacyDefaultFalse(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml")
	doc, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.Check(doc, filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if doc.Spec.Management.MCP.AllowLegacyClients {
		t.Fatal("product default allowLegacyClients must be false")
	}
}

func TestJungleOverlayAllowLegacy(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata", "mcp", "jungle-overlay.yaml")
	doc, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.Check(doc, filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if !doc.Spec.Management.MCP.AllowLegacyClients {
		t.Fatal("MCPJungle overlay must set allowLegacyClients true")
	}
	lab := filepath.Join(repoRoot(t), "testdata", "config", "valid", "lab-overlay.yaml")
	labDoc, err := config.LoadFile(lab)
	if err != nil {
		t.Fatal(err)
	}
	if !labDoc.Spec.Management.MCP.AllowLegacyClients {
		t.Fatal("lab-overlay.yaml must set allowLegacyClients true")
	}
}

func TestMethodNotPost(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(headerAuthorization, "Bearer "+testBearerSecret)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405 got %d", w.Code)
	}
}

func TestOverlayFixtureExists(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata", "mcp", "jungle-overlay.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
