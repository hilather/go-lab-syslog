package config

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

const minimal = `
apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec: {}
`

func TestDecodeRejectsEmpty(t *testing.T) {
	if _, err := DecodeYAML(nil); !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("nil: %v", err)
	}
	if _, err := DecodeYAML([]byte("   \n")); !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("whitespace: %v", err)
	}
}

func TestDecodeRejectsNonUTF8(t *testing.T) {
	if _, err := DecodeYAML([]byte{0xff, 0xfe, 0x00}); !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeRejectsOversize(t *testing.T) {
	data := bytes.Repeat([]byte("a"), MaxDocumentBytes+1)
	if _, err := DecodeYAML(data); !domainerr.Is(err, domainerr.PayloadTooLarge) {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeRejectsMultiDoc(t *testing.T) {
	data := []byte(minimal + "---\napiVersion: labsyslog.dev/v1alpha1\nkind: LabSyslog\nmetadata:\n  name: other\nspec: {}\n")
	if _, err := DecodeYAML(data); !domainerr.Is(err, domainerr.ValidationFailed) || !strings.Contains(err.Error(), "multi-document") {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeJSONUnknownField(t *testing.T) {
	raw := []byte(`{"apiVersion":"labsyslog.dev/v1alpha1","kind":"LabSyslog","metadata":{"name":"lab-sink"},"spec":{"nope":true}}`)
	if _, err := DecodeJSON(raw); !domainerr.Is(err, domainerr.UnknownField) {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeJSONOK(t *testing.T) {
	raw := []byte(`{"apiVersion":"labsyslog.dev/v1alpha1","kind":"LabSyslog","metadata":{"name":"lab-sink"},"spec":{}}`)
	doc, err := DecodeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Metadata.Name != "lab-sink" {
		t.Fatalf("name %q", doc.Metadata.Name)
	}
}

func TestMinimalYAML(t *testing.T) {
	doc, err := DecodeYAML([]byte(minimal))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Kind != "LabSyslog" {
		t.Fatalf("kind %q", doc.Kind)
	}
}
