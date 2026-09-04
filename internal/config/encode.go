package config

import (
	"bytes"

	"github.com/hilather/go-lab-syslog/internal/model"
	"gopkg.in/yaml.v3"
)

// EncodeCanonical marshals a normalized document as canonical YAML.
// Fields follow struct order. Secret paths are included; secret bytes are not.
func EncodeCanonical(doc *model.Document) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
