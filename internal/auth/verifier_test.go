package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestStaticBearer(t *testing.T) {
	v := Static(testSecret, "operator", RoleAdministrator)
	p, err := v.Authenticate(Request{Authorization: "Bearer " + testSecret})
	if err != nil || p.ID != "operator" {
		t.Fatalf("%+v %v", p, err)
	}
	if !p.HasScope(ScopeAdmin) || !p.HasScope(ScopeRead) || !p.HasScope(ScopeWrite) || !p.HasScope(ScopeAuditRead) {
		t.Fatalf("admin scopes %v", p.Scopes)
	}
	_, err = v.Authenticate(Request{Authorization: "Basic dXNlcjpwYXNz"})
	if err == nil {
		t.Fatal("Basic must 401")
	}
	if !domainerr.Is(err, domainerr.Unauthorized) {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(WWWAuthenticate(), "Bearer") {
		t.Fatal(WWWAuthenticate())
	}
}

func TestReaderScopes(t *testing.T) {
	v := Static(testSecret, "r", RoleReader)
	p, err := v.AuthenticateBearer(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if p.HasScope(ScopeAdmin) || p.HasScope(ScopeWrite) || p.HasScope(ScopeAuditRead) {
		t.Fatalf("reader scopes %v", p.Scopes)
	}
	if !p.HasScope(ScopeRead) {
		t.Fatal("reader missing syslog.read")
	}
	if err := Authorize(p, ScopeAdmin); !domainerr.Is(err, domainerr.Forbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestFromSpecFileRefAndMinBytes(t *testing.T) {
	dir := t.TempDir()
	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, []byte("tooshort\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := FromSpec(model.Auth{
		Mode: "bearer",
		Tokens: []model.AuthToken{{
			ID: "op", Role: RoleAdministrator, SecretFile: short,
		}},
	}, dir)
	if err == nil {
		t.Fatal("short token")
	}
	okPath := filepath.Join(dir, "ok")
	if err := os.WriteFile(okPath, []byte(testSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := FromSpec(model.Auth{
		Mode: "bearer",
		Tokens: []model.AuthToken{{
			ID: "op", Role: RoleAdministrator, SecretFile: okPath,
		}},
	}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if v.TokenCount() != 1 {
		t.Fatal(v.TokenCount())
	}
}

func TestFromSpecMissingFileSkipped(t *testing.T) {
	v, err := FromSpec(model.Auth{
		Mode: "bearer",
		Tokens: []model.AuthToken{{
			ID: "op", Role: RoleAdministrator, SecretFile: "missing.token",
		}},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if v.TokenCount() != 0 {
		t.Fatal(v.TokenCount())
	}
}

func TestMissingAuthUnauthenticated(t *testing.T) {
	v := Static(testSecret, "op", RoleAdministrator)
	_, err := v.Authenticate(Request{})
	if !domainerr.Is(err, domainerr.Unauthorized) {
		t.Fatalf("%v", err)
	}
}
