package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/config"
	"gopkg.in/yaml.v3"
)

func TestLabOverlayYAML(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "labsyslog.yaml")
	doc, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := compiler.Check(doc, filepath.Dir(path)); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if doc.APIVersion != "labsyslog.dev/v1alpha1" || doc.Kind != "LabSyslog" {
		t.Fatalf("identity %s %s", doc.APIVersion, doc.Kind)
	}
	if doc.Spec.Listeners.UDP.Enabled == nil || !*doc.Spec.Listeners.UDP.Enabled {
		t.Fatal("udp.enabled must be true")
	}
	if doc.Spec.Listeners.TCP.Enabled == nil || !*doc.Spec.Listeners.TCP.Enabled {
		t.Fatal("tcp.enabled must be true")
	}
	if doc.Spec.Listeners.UDP.Address != ":514" {
		t.Fatalf("udp address %q, want :514", doc.Spec.Listeners.UDP.Address)
	}
	if doc.Spec.Listeners.TCP.Address != ":514" {
		t.Fatalf("tcp address %q, want :514", doc.Spec.Listeners.TCP.Address)
	}
	if len(doc.Spec.Filters) != 0 {
		t.Fatalf("lab overlay filters %d, want capture-all empty", len(doc.Spec.Filters))
	}
	if doc.Spec.Listeners.TLS.Enabled {
		t.Fatal("tls.enabled must be false in 1.0")
	}
	if doc.Spec.Auth.Mode != "bearer" {
		t.Fatalf("auth.mode %q", doc.Spec.Auth.Mode)
	}
	if !doc.Spec.Management.MCP.AllowLegacyClients {
		t.Fatal("lab overlay must set allowLegacyClients true")
	}
	if !containsString(doc.Spec.Admission.AllowClientCIDRs, "10.99.42.0/24") {
		t.Fatalf("allowClientCidrs %v missing lab subnet", doc.Spec.Admission.AllowClientCIDRs)
	}
}

func TestLabinfoYAML(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "labinfo", "services-labsyslog.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services []map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Services) != 1 {
		t.Fatalf("want 1 service, got %d", len(doc.Services))
	}
	svc := doc.Services[0]
	id, _ := svc["id"].(string)
	if id != "labsyslog" {
		t.Fatalf("labinfo id %q, want labsyslog", id)
	}
	conn, _ := svc["connection"].(map[string]any)
	if conn == nil {
		t.Fatal("labinfo entry must have a connection block")
	}
	endpoints, _ := conn["endpoints"].([]any)
	if len(endpoints) < 3 {
		t.Fatalf("connection.endpoints %v, want udp + tcp + mcp", endpoints)
	}
}

func TestMCPJungleJSON(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "mcpjungle", "servers", "labsyslog.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	name, _ := doc["name"].(string)
	if name != "labsyslog" {
		t.Fatalf("name %q, want labsyslog", name)
	}
	if filepath.Base(path) != name+".json" {
		t.Fatalf("filename %s must equal name %s.json", filepath.Base(path), name)
	}
	url, _ := doc["url"].(string)
	if url != "http://labsyslog:8088/mcp" {
		t.Fatalf("url %q", url)
	}
	token, _ := doc["bearer_token"].(string)
	if token != "${LABSYSLOG_TOKEN}" {
		t.Fatalf("bearer_token %q", token)
	}
	transport, _ := doc["transport"].(string)
	if transport != "streamable_http" {
		t.Fatalf("transport %q", transport)
	}
}

func TestDocs13ComposeNETBindService(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "13-integration-lab-swap.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "cap_add: [NET_BIND_SERVICE]") {
		t.Fatal("docs/13 compose must include cap_add: [NET_BIND_SERVICE] for container :514")
	}
	if !strings.Contains(text, "LABSYSLOG_REST_PORT") {
		t.Fatal("docs/13 must keep LABSYSLOG_REST_PORT (C24)")
	}
	if !strings.Contains(text, "syslog_messages_wait") {
		t.Fatal("docs/13 smoke recipe must use syslog_messages_wait")
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "10514 does not need") || strings.Contains(lower, "residual 10514 does not need") {
		t.Fatal("docs/13 must not claim residual 10514 needs no cap (C17)")
	}
	if strings.Contains(text, "only when the profile publishes host 514") {
		t.Fatal("docs/13 must add NET_BIND_SERVICE whenever the process binds :514, including 10514→514")
	}
}

func TestDocs13LabinfoIDs(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "13-integration-lab-swap.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	// Current mcp-integration-lab catalog ids we must not collide with.
	ids := []string{
		"gateway", "labdns", "labldap", "labtacacs", "maildev",
		"nfs", "labmitm", "labgraph", "labntp", "labsso",
	}
	for _, id := range ids {
		if !strings.Contains(text, "`"+id+"`") {
			t.Errorf("docs/13 must mention labinfo id %q we must not collide with", id)
		}
	}
	if !strings.Contains(text, "`labsyslog`") {
		t.Fatal("docs/13 must mention catalog id labsyslog")
	}
}
