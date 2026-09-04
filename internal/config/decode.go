package config

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"gopkg.in/yaml.v3"
)

// MaxDocumentBytes is the fail-closed YAML/JSON size cap (1 MiB).
const MaxDocumentBytes = 1 << 20

// LoadFile reads one fail-closed YAML document from path.
func LoadFile(path string) (*model.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodeYAML(data)
}

// DecodeYAML decodes one labsyslog.dev/v1alpha1 document.
func DecodeYAML(data []byte) (*model.Document, error) {
	if err := checkDocumentBytes(data); err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, mapYAMLError(err)
	}
	if isEmptyYAML(&root) {
		return nil, domainerr.New(domainerr.ValidationFailed, "document is empty")
	}
	if err := walkYAMLKeys(&root, map[*yaml.Node]bool{}, checkKey); err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var doc model.Document
	if err := dec.Decode(&doc); err != nil {
		return nil, mapYAMLError(err)
	}
	var extra yaml.Node
	err := dec.Decode(&extra)
	switch {
	case err == nil:
		return nil, domainerr.New(domainerr.ValidationFailed, "multi-document YAML is not allowed")
	case err != io.EOF:
		return nil, mapYAMLError(err)
	}
	return &doc, nil
}

// DecodeJSON decodes one document with DisallowUnknownFields.
func DecodeJSON(data []byte) (*model.Document, error) {
	if err := checkDocumentBytes(data); err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, domainerr.New(domainerr.ValidationFailed, "document is empty")
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, domainerr.New(domainerr.ValidationFailed, err.Error())
	}
	if raw == nil {
		return nil, domainerr.New(domainerr.ValidationFailed, "document is empty")
	}
	if err := walkJSONKeys(raw, checkKey); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc model.Document
	if err := dec.Decode(&doc); err != nil {
		return nil, mapJSONError(err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, domainerr.New(domainerr.ValidationFailed, "trailing JSON after document")
	}
	return &doc, nil
}

func checkDocumentBytes(data []byte) error {
	if len(data) > MaxDocumentBytes {
		return domainerr.New(domainerr.PayloadTooLarge, "document exceeds 1 MiB")
	}
	if !utf8.Valid(data) {
		return domainerr.New(domainerr.ValidationFailed, "document is not UTF-8")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return domainerr.New(domainerr.ValidationFailed, "document is empty")
	}
	return nil
}

func isEmptyYAML(n *yaml.Node) bool {
	if n == nil {
		return true
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return true
		}
		return isEmptyYAML(n.Content[0])
	case 0:
		return n.Tag == "" && n.Value == "" && len(n.Content) == 0
	case yaml.ScalarNode:
		return n.Value == "" || n.Tag == "!!null"
	default:
		return false
	}
}

func walkYAMLKeys(n *yaml.Node, seen map[*yaml.Node]bool, fn func(string) error) error {
	if n == nil || seen[n] {
		return nil
	}
	seen[n] = true
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			if err := walkYAMLKeys(c, seen, fn); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Kind == yaml.ScalarNode {
				if err := fn(k.Value); err != nil {
					return err
				}
			}
			if err := walkYAMLKeys(v, seen, fn); err != nil {
				return err
			}
		}
	case yaml.AliasNode:
		return walkYAMLKeys(n.Alias, seen, fn)
	}
	return nil
}

func walkJSONKeys(v any, fn func(string) error) error {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if err := fn(k); err != nil {
				return err
			}
			if err := walkJSONKeys(val, fn); err != nil {
				return err
			}
		}
	case []any:
		for _, val := range t {
			if err := walkJSONKeys(val, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

func mapYAMLError(err error) error {
	if err == nil {
		return nil
	}
	if err == io.EOF {
		return domainerr.New(domainerr.ValidationFailed, "document is empty")
	}
	msg := err.Error()
	if strings.Contains(msg, "not found in type") {
		return domainerr.New(domainerr.UnknownField, msg)
	}
	return domainerr.New(domainerr.ValidationFailed, msg)
}

func mapJSONError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown field") {
		return domainerr.New(domainerr.UnknownField, msg)
	}
	return domainerr.New(domainerr.ValidationFailed, msg)
}
