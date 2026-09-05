package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/model"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestValidateDocumentObject(t *testing.T) {
	s, _ := newTestServer(t)
	cs := connectClient(t, startHTTP(t, s))
	res := callTool(t, cs, "syslog_state_validate", map[string]any{
		"document": map[string]any{
			"apiVersion": "labsyslog.dev/v1alpha1",
			"kind":       "LabSyslog",
			"metadata":   map[string]any{"name": "lab-sink"},
			"spec":       map[string]any{},
		},
	})
	if res.IsError {
		t.Fatalf("validate structured=%+v content=%s", res.StructuredContent, textOf(res))
	}
	m, _ := res.StructuredContent.(map[string]any)
	if m["valid"] != true {
		t.Fatalf("valid %v", m["valid"])
	}
}

func TestApplyReplaceStoreCapsStringSizes(t *testing.T) {
	s, svc := newTestServer(t)
	cs := connectClient(t, startHTTP(t, s))
	rev := svc.State(t.Context()).Revision
	res := callTool(t, cs, "syslog_change_apply", map[string]any{
		"expectedRevision": rev,
		"idempotencyKey":   "apply-store-caps-strings",
		"operations": []any{
			map[string]any{
				"type": app.OpReplaceStoreCaps,
				"store": map[string]any{
					"maxMessages": 10000,
					"maxBytes":    "256MiB",
					"fullPolicy":  "evict_oldest",
					"maxWait":     "60s",
					"rawRetain":   true,
				},
			},
		},
	})
	if res.IsError {
		t.Fatalf("apply %+v content=%+v", res.StructuredContent, res.Content)
	}
	st := svc.Snapshot().Document.Spec.Store
	if st.MaxBytes != 256*model.MiB {
		t.Fatalf("maxBytes %s", st.MaxBytes)
	}
	if st.MaxWait.Duration().Seconds() != 60 {
		t.Fatalf("maxWait %s", st.MaxWait)
	}
}

func TestToolsListSchemasAreObjectsAndStrings(t *testing.T) {
	s, _ := newTestServer(t)
	cs := connectClient(t, startHTTP(t, s))
	var validateSchema, applySchema any
	for tool, err := range cs.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if strings.Contains(text, `"minItems":0`) && strings.Contains(text, `"maxItems":255`) {
			t.Errorf("%s schema looks like a byte array: %s", tool.Name, text)
		}
		switch tool.Name {
		case "syslog_state_validate":
			validateSchema = tool.InputSchema
		case "syslog_change_apply":
			applySchema = tool.InputSchema
		}
	}
	if validateSchema == nil || applySchema == nil {
		t.Fatal("missing validate/apply schemas")
	}
	docType := schemaAt(t, validateSchema, "properties", "document", "type")
	if !schemaTypeIs(docType, "object") {
		t.Fatalf("document type %v want object", docType)
	}
	maxBytes := schemaAt(t, applySchema, "properties", "operations", "items", "properties", "store", "properties", "maxBytes", "type")
	if !schemaTypeIs(maxBytes, "string") {
		t.Fatalf("store.maxBytes type %v want string", maxBytes)
	}
	maxWait := schemaAt(t, applySchema, "properties", "operations", "items", "properties", "store", "properties", "maxWait", "type")
	if !schemaTypeIs(maxWait, "string") {
		t.Fatalf("store.maxWait type %v want string", maxWait)
	}
}

func schemaAt(t *testing.T, root any, path ...string) any {
	t.Helper()
	cur := root
	for _, p := range path {
		m, ok := asMap(cur)
		if !ok {
			t.Fatalf("not object at %v: %T", path, cur)
		}
		cur, ok = m[p]
		if !ok {
			t.Fatalf("missing %q in %v", p, path)
		}
	}
	return cur
}

func asMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false
	}
	return m, true
}

func textOf(res *sdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func schemaTypeIs(v any, want string) bool {
	switch t := v.(type) {
	case string:
		return t == want
	case []any:
		for _, x := range t {
			s, _ := x.(string)
			if s == want {
				return true
			}
		}
	}
	return false
}
