package mcp

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hilather/go-lab-syslog/internal/model"
)

func schemaOptions() *jsonschema.ForOptions {
	str := &jsonschema.Schema{Type: "string"}
	return &jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[model.Duration](): str,
			reflect.TypeFor[model.ByteSize](): str,
		},
	}
}

func inputSchemaFor[T any]() (*jsonschema.Schema, error) {
	s, err := jsonschema.For[T](schemaOptions())
	if err != nil {
		return nil, err
	}
	// REST KnownFields decode treats omitted fields as zero values. jsonschema.For
	// marks every non-omitempty struct field required, which would reject the
	// same documents REST accepts (spec: {}).
	relaxRequired(s)
	return s, nil
}

func relaxRequired(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	s.Required = nil
	for _, p := range s.Properties {
		relaxRequired(p)
	}
	relaxRequired(s.Items)
	for _, p := range s.PrefixItems {
		relaxRequired(p)
	}
	relaxRequired(s.AdditionalProperties)
	for _, p := range s.Defs {
		relaxRequired(p)
	}
}

func mustInputSchema[T any](name string) *jsonschema.Schema {
	s, err := inputSchemaFor[T]()
	if err != nil {
		panic("mcp: input schema " + name + ": " + err.Error())
	}
	return s
}

// ToolInputSchemas infers JSON Schema for every syslog_* tool input from the
// same Go types AddTool uses.
func ToolInputSchemas() (map[string]any, error) {
	type named struct {
		name string
		fn   func() (*jsonschema.Schema, error)
	}
	entries := []named{
		{"syslog_version_get", func() (*jsonschema.Schema, error) { return inputSchemaFor[emptyIn]() }},
		{"syslog_capabilities_get", func() (*jsonschema.Schema, error) { return inputSchemaFor[emptyIn]() }},
		{"syslog_status_get", func() (*jsonschema.Schema, error) { return inputSchemaFor[emptyIn]() }},
		{"syslog_schema_get", func() (*jsonschema.Schema, error) { return inputSchemaFor[emptyIn]() }},
		{"syslog_features_list", func() (*jsonschema.Schema, error) { return inputSchemaFor[emptyIn]() }},
		{"syslog_state_get", func() (*jsonschema.Schema, error) { return inputSchemaFor[emptyIn]() }},
		{"syslog_state_validate", func() (*jsonschema.Schema, error) { return inputSchemaFor[validateIn]() }},
		{"syslog_state_export", func() (*jsonschema.Schema, error) { return inputSchemaFor[exportIn]() }},
		{"syslog_state_reset", func() (*jsonschema.Schema, error) { return inputSchemaFor[reasonIn]() }},
		{"syslog_change_plan", func() (*jsonschema.Schema, error) { return inputSchemaFor[changeIn]() }},
		{"syslog_change_apply", func() (*jsonschema.Schema, error) { return inputSchemaFor[changeIn]() }},
		{"syslog_messages_list", func() (*jsonschema.Schema, error) { return inputSchemaFor[messagesListIn]() }},
		{"syslog_message_get", func() (*jsonschema.Schema, error) { return inputSchemaFor[messageGetIn]() }},
		{"syslog_message_raw_get", func() (*jsonschema.Schema, error) { return inputSchemaFor[idIn]() }},
		{"syslog_message_delete", func() (*jsonschema.Schema, error) { return inputSchemaFor[idIn]() }},
		{"syslog_messages_clear", func() (*jsonschema.Schema, error) { return inputSchemaFor[reasonIn]() }},
		{"syslog_messages_wait", func() (*jsonschema.Schema, error) { return inputSchemaFor[waitIn]() }},
		{"syslog_stats_get", func() (*jsonschema.Schema, error) { return inputSchemaFor[emptyIn]() }},
		{"syslog_audit_query", func() (*jsonschema.Schema, error) { return inputSchemaFor[emptyIn]() }},
		{"syslog_audit_get", func() (*jsonschema.Schema, error) { return inputSchemaFor[idIn]() }},
	}
	out := make(map[string]any, len(entries))
	for _, e := range entries {
		s, err := e.fn()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.name, err)
		}
		raw, err := json.Marshal(s)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.name, err)
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("%s: %w", e.name, err)
		}
		out[e.name] = v
	}
	return out, nil
}
