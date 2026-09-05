// Command generatejsonschema writes generated API artifacts (JSON Schema,
// OpenAPI, error catalog, capability table, metrics catalog, MCP manifest).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hilather/go-lab-syslog/internal/control/mcp"
)

const (
	relPath         = "api/jsonschema/labsyslog.dev.v1alpha1.json"
	relOpenAPI      = "api/openapi/v1.json"
	relErrors       = "api/errors/v1.json"
	relCapabilities = "api/capabilities/v1.json"
	relMetrics      = "api/metrics/v1alpha1.json"
	relMCP          = mcp.ManifestRelPath
)

func main() {
	check := flag.Bool("check", false, "verify generated file matches")
	flag.Parse()
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generatejsonschema: %v\n", err)
		os.Exit(1)
	}
	if *check {
		if err := Check(root); err != nil {
			fmt.Fprintf(os.Stderr, "generatejsonschema: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := Generate(root); err != nil {
		fmt.Fprintf(os.Stderr, "generatejsonschema: %v\n", err)
		os.Exit(1)
	}
}

// Generate writes JSON Schema, OpenAPI, error catalog, capabilities, metrics, and MCP manifest.
func Generate(root string) error {
	for _, art := range artifacts() {
		body, err := artBytes(art)
		if err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(art.rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Check fails if any generated artifact is stale.
func Check(root string) error {
	for _, art := range artifacts() {
		want, err := artBytes(art)
		if err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(art.rel))
		got, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w (run make generate)", art.rel, err)
		}
		if !bytes.Equal(want, got) {
			return fmt.Errorf("%s is stale; run make generate", art.rel)
		}
	}
	return nil
}

func artBytes(art artifact) ([]byte, error) {
	if art.raw != nil {
		return art.raw, nil
	}
	return renderValue(art.value)
}

type artifact struct {
	rel   string
	value any
	raw   []byte
}

func artifacts() []artifact {
	mcpRaw, err := mcp.RenderManifest()
	if err != nil {
		panic(err)
	}
	return []artifact{
		{rel: relPath, value: schema()},
		{rel: relOpenAPI, value: openAPI()},
		{rel: relErrors, value: errorsCatalog()},
		{rel: relCapabilities, value: capabilitiesCatalog()},
		{rel: relMetrics, value: metricsCatalog()},
		{rel: relMCP, raw: mcpRaw},
	}
}

func render() ([]byte, error) {
	return renderValue(schema())
}

func renderValue(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

type obj []kv

type kv struct {
	k string
	v any
}

func (o obj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, p := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := json.Marshal(p.k)
		if err != nil {
			return nil, err
		}
		vb, err := json.Marshal(p.v)
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func schema() obj {
	stringT := obj{{"type", "string"}}
	duration := obj{{"type", "string"}, {"description", "Go duration string (2m, 60s, 500ms)"}}
	byteSize := obj{{"type", "string"}, {"description", "binary byte size (64KiB, 256MiB, 1MiB)"}}
	addr := obj{{"type", "string"}, {"description", "Go listen address (:514, 127.0.0.1:1514)"}}

	udp := closedObj([]string{}, obj{
		{"enabled", obj{{"type", "boolean"}, {"default", true}}},
		{"address", addr},
	})
	tcp := closedObj([]string{}, obj{
		{"enabled", obj{{"type", "boolean"}, {"default", true}}},
		{"address", addr},
		{"framing", obj{{"type", "string"}, {"enum", []string{"auto", "octet-counting", "non-transparent"}}, {"default", "auto"}}},
	})
	tls := closedObj([]string{}, obj{
		{"enabled", obj{{"type", "boolean"}, {"const", false}, {"default", false}, {"description", "true is tls_unsupported in 1.0 (ADR 0012)"}}},
		{"address", addr},
		{"certFile", stringT},
		{"keyFile", stringT},
		{"caFile", stringT},
		{"clientAuth", obj{{"type", "boolean"}, {"default", false}}},
	})
	mgmtListener := closedObj([]string{}, obj{
		{"address", addr},
		{"restPath", obj{{"type", "string"}, {"default", "/v1"}}},
		{"mcpPath", obj{{"type", "string"}, {"default", "/mcp"}}},
	})
	listeners := closedObj([]string{}, obj{
		{"udp", udp},
		{"tcp", tcp},
		{"tls", tls},
		{"management", mgmtListener},
	})
	token := closedObj([]string{"id", "secretFile"}, obj{
		{"id", stringT},
		{"role", obj{{"type", "string"}, {"enum", []string{"administrator", "reader"}}, {"default", "administrator"}}},
		{"secretFile", obj{{"type", "string"}, {"description", "path to token file; contents trimmed, ≥32 bytes if the file exists"}}},
	})
	auth := closedObj([]string{}, obj{
		{"mode", obj{{"type", "string"}, {"const", "bearer"}, {"default", "bearer"}}},
		{"tokens", obj{{"type", "array"}, {"items", token}}},
	})
	ui := closedObj([]string{}, obj{
		{"enabled", obj{{"type", "boolean"}, {"default", true}}},
	})
	mcp := closedObj([]string{}, obj{
		{"allowLegacyClients", obj{{"type", "boolean"}, {"default", false}}},
	})
	management := closedObj([]string{}, obj{
		{"allowedOrigins", obj{{"type", "array"}, {"items", stringT}, {"description", "exact origins; empty is loopback only; non-empty is exactly the list (loopback not unioned); originAllowlist rejects"}}},
		{"mcp", mcp},
		{"bodyLimit", obj{{"type", "string"}, {"default", "1MiB"}}},
		{"requestsPerSecond", obj{{"type", "integer"}, {"default", 32}}},
		{"burst", obj{{"type", "integer"}, {"default", 64}}},
		{"maxConcurrent", obj{{"type", "integer"}, {"default", 256}}},
	})
	parse := closedObj([]string{}, obj{
		{"rfc3164", obj{{"type", "boolean"}, {"default", true}}},
		{"rfc5424", obj{{"type", "boolean"}, {"default", true}}},
		{"bestEffort", obj{{"type", "boolean"}, {"default", true}}},
	})
	syslog := closedObj([]string{}, obj{
		{"hostname", obj{{"type", "string"}, {"default", "labsyslog.lab"}}},
		{"parse", parse},
		{"maxMessageBytes", byteSize},
		{"udpMaxDatagramBytes", byteSize},
		{"tcpIdleTimeout", duration},
		{"behavior", closedObj([]string{}, obj{
			{"mode", obj{{"type", "string"}, {"enum", []string{"accept", "drop-silent", "close"}}, {"default", "accept"}}},
		})},
	})
	admission := closedObj([]string{}, obj{
		{"allowClientCidrs", obj{{"type", "array"}, {"items", stringT}, {"minItems", 1}, {"description", "omit for loopback default; empty list is a validate error"}}},
		{"maxDatagramsPerSec", obj{{"type", "integer"}, {"default", 20000}}},
		{"maxDatagramsPerIP", obj{{"type", "integer"}, {"default", 2000}}},
		{"maxTcpConns", obj{{"type", "integer"}, {"default", 256}}},
		{"maxTcpConnsPerIP", obj{{"type", "integer"}, {"default", 16}}},
		{"sessionTimeout", duration},
	})
	store := closedObj([]string{}, obj{
		{"maxMessages", obj{{"type", "integer"}, {"minimum", 1}, {"maximum", 1000000}, {"default", 10000}}},
		{"maxBytes", byteSize},
		{"fullPolicy", obj{{"type", "string"}, {"enum", []string{"evict_oldest", "reject"}}, {"default", "evict_oldest"}}},
		{"maxWait", duration},
		{"rawRetain", obj{{"type", "boolean"}, {"default", true}}},
	})
	transport := obj{{"type", "string"}, {"enum", []string{"udp", "tcp"}}}
	filterMatch := closedObj([]string{}, obj{
		{"sourceCidrs", obj{{"type", "array"}, {"items", stringT}}},
		{"facilities", obj{{"type", "array"}, {"items", stringT}}},
		{"severities", obj{{"type", "array"}, {"items", stringT}}},
		{"severityAtLeast", stringT},
		{"appNames", obj{{"type", "array"}, {"items", stringT}}},
		{"hostnames", obj{{"type", "array"}, {"items", stringT}}},
		{"transports", obj{{"type", "array"}, {"items", transport}}},
	})
	filterAction := closedObj([]string{}, obj{
		{"mode", obj{{"type", "string"}, {"enum", []string{"capture", "drop-silent", "tag"}}}},
		{"tag", stringT},
	})
	filter := closedObj([]string{"name"}, obj{
		{"name", obj{{"type", "string"}, {"description", "DNS label; unique"}}},
		{"enabled", obj{{"type", "boolean"}, {"default", true}}},
		{"match", filterMatch},
		{"action", filterAction},
	})
	observability := closedObj([]string{}, obj{
		{"logLevel", obj{{"type", "string"}, {"enum", []string{"debug", "info", "warn", "error"}}, {"default", "info"}}},
		{"metrics", closedObj([]string{}, obj{
			{"publicPath", obj{{"type", "boolean"}, {"default", false}}},
		})},
		{"audit", closedObj([]string{}, obj{
			{"ring", obj{{"type", "integer"}, {"default", 128}}},
		})},
	})
	spec := closedObj([]string{}, obj{
		{"listeners", listeners},
		{"auth", auth},
		{"ui", ui},
		{"management", management},
		{"syslog", syslog},
		{"admission", admission},
		{"store", store},
		{"filters", obj{{"type", "array"}, {"items", filter}}},
		{"observability", observability},
	})
	return obj{
		{"$schema", "https://json-schema.org/draft/2020-12/schema"},
		{"$id", "https://labsyslog.dev/schemas/labsyslog.dev.v1alpha1.json"},
		{"title", "LabSyslog"},
		{"type", "object"},
		{"additionalProperties", false},
		{"required", []string{"apiVersion", "kind", "metadata", "spec"}},
		{"properties", obj{
			{"apiVersion", obj{{"type", "string"}, {"const", "labsyslog.dev/v1alpha1"}}},
			{"kind", obj{{"type", "string"}, {"const", "LabSyslog"}}},
			{"metadata", closedObj([]string{"name"}, obj{
				{"name", obj{{"type", "string"}, {"pattern", "^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$"}}},
			})},
			{"spec", spec},
		}},
	}
}

func closedObj(required []string, properties obj) obj {
	out := obj{
		{"type", "object"},
		{"additionalProperties", false},
	}
	if len(required) > 0 {
		out = append(out, kv{"required", required})
	}
	out = append(out, kv{"properties", properties})
	return out
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}
