package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/config"
	"gopkg.in/yaml.v3"
)

func TestSmokeOverlayYAML(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata", "container", "config.yaml")
	doc, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := compiler.Check(doc, filepath.Dir(path)); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if doc.Spec.Listeners.UDP.Address != ":1514" {
		t.Fatalf("udp address %q, want :1514", doc.Spec.Listeners.UDP.Address)
	}
	if doc.Spec.Listeners.TCP.Address != ":1514" {
		t.Fatalf("tcp address %q, want :1514", doc.Spec.Listeners.TCP.Address)
	}
	if doc.Spec.Listeners.Management.Address != ":8088" {
		t.Fatalf("management address %q, want :8088", doc.Spec.Listeners.Management.Address)
	}
	if doc.Spec.Auth.Mode != "bearer" {
		t.Fatalf("auth.mode %q", doc.Spec.Auth.Mode)
	}
	if len(doc.Spec.Auth.Tokens) != 1 || doc.Spec.Auth.Tokens[0].SecretFile != "/run/secrets/labsyslog-token" {
		t.Fatalf("token mount %+v", doc.Spec.Auth.Tokens)
	}
}

func TestComposeSmokeYAML(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "compose.smoke.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	svc := composeService(t, doc, "labsyslog")
	if _, ok := svc["cap_add"]; ok {
		t.Fatal("compose.smoke.yaml must not cap_add (no NET_BIND_SERVICE)")
	}
	ports := stringSlice(t, svc["ports"])
	wantPorts := []string{
		"127.0.0.1:1514:1514/udp",
		"127.0.0.1:1514:1514/tcp",
		"127.0.0.1:18088:8088",
	}
	for _, p := range wantPorts {
		if !containsString(ports, p) {
			t.Fatalf("ports %v missing %q", ports, p)
		}
	}
	caps := stringSlice(t, svc["cap_drop"])
	if !containsString(caps, "ALL") {
		t.Fatalf("cap_drop %v missing ALL", caps)
	}
	if user, _ := svc["user"].(string); user != "65532:65532" {
		t.Fatalf("user %v, want 65532:65532", svc["user"])
	}
	if ro, _ := svc["read_only"].(bool); !ro {
		t.Fatal("read_only must be true")
	}
	hc, _ := svc["healthcheck"].(map[string]any)
	test := stringSlice(t, hc["test"])
	joined := strings.Join(test, " ")
	if len(test) == 0 || test[0] != "CMD" {
		t.Fatalf("healthcheck.test %v must be exec-form CMD", test)
	}
	if !strings.Contains(joined, "healthcheck") || !strings.Contains(joined, "/v1/health/ready") {
		t.Fatalf("healthcheck.test %v want labsyslog healthcheck ready", test)
	}
}

func TestDockerfileContract(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"FROM scratch",
		"USER 65532:65532",
		`CMD ["serve", "--config=/etc/labsyslog/config.yaml", "--management-listen=:8088"]`,
		`CMD ["/labsyslog", "healthcheck", "--url=http://127.0.0.1:8088/v1/health/ready"]`,
		"EXPOSE 514/udp 514/tcp 8088/tcp",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Dockerfile missing %q", want)
		}
	}
	if strings.Contains(text, "CMD-SHELL") {
		t.Fatal("HEALTHCHECK must stay exec form")
	}
	if strings.Contains(text, "USER 0") || strings.Contains(text, "USER root") {
		t.Fatal("image user must stay 65532")
	}
}

func composeService(t *testing.T, doc map[string]any, name string) map[string]any {
	t.Helper()
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		t.Fatal("compose missing services")
	}
	svc, _ := services[name].(map[string]any)
	if svc == nil {
		t.Fatalf("compose missing service %s", name)
	}
	return svc
}

func stringSlice(t *testing.T, v any) []string {
	t.Helper()
	switch x := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				t.Fatalf("expected string list, got %T", item)
			}
			out = append(out, s)
		}
		return out
	default:
		t.Fatalf("expected list, got %T", v)
		return nil
	}
}

func containsString(items []string, want string) bool {
	for _, s := range items {
		if s == want {
			return true
		}
	}
	return false
}
