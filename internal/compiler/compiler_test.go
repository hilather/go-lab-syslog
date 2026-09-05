package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

func TestRevisionStableAndIgnoresSecretBytes(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, bytes.Repeat([]byte("a"), auth.MinTokenBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml1 := []byte(`
apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec:
  auth:
    tokens:
      - id: operator
        secretFile: ` + tok + `
`)
	doc, err := config.DecodeYAML(yaml1)
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(doc, dir); err != nil {
		t.Fatal(err)
	}
	canon, err := config.EncodeCanonical(doc)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(canon, bytes.Repeat([]byte("a"), auth.MinTokenBytes)) {
		t.Fatal("secret bytes entered canonical YAML")
	}
	if !bytes.Contains(canon, []byte(tok)) && !bytes.Contains(canon, []byte("secretFile")) {
		t.Fatalf("secret path missing from canonical YAML:\n%s", canon)
	}
	rev1 := Revision(canon)

	if err := os.WriteFile(tok, bytes.Repeat([]byte("b"), auth.MinTokenBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	doc2, err := config.DecodeYAML(yaml1)
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(doc2, dir); err != nil {
		t.Fatal(err)
	}
	canon2, err := config.EncodeCanonical(doc2)
	if err != nil {
		t.Fatal(err)
	}
	rev2 := Revision(canon2)
	if rev1 != rev2 {
		t.Fatalf("revision changed when secret bytes changed: %s vs %s", rev1, rev2)
	}

	other := filepath.Join(dir, "other-token")
	if err := os.WriteFile(other, bytes.Repeat([]byte("a"), auth.MinTokenBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	yamlOther := bytes.ReplaceAll(yaml1, []byte(tok), []byte(other))
	doc3, err := config.DecodeYAML(yamlOther)
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(doc3, dir); err != nil {
		t.Fatal(err)
	}
	canon3, err := config.EncodeCanonical(doc3)
	if err != nil {
		t.Fatal(err)
	}
	if Revision(canon3) == rev1 {
		t.Fatal("revision must change when secret path changes")
	}
}

func TestEmptyCIDRsRejectedAfterNormalize(t *testing.T) {
	doc := &model.Document{
		APIVersion: model.APIVersion,
		Kind:       model.Kind,
		Metadata:   model.Metadata{Name: "lab-sink"},
	}
	doc.Spec.Admission.AllowClientCIDRs = []string{}
	if err := Check(doc, ""); !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("got %v", err)
	}
}

func TestOmittedCIDRsDefault(t *testing.T) {
	doc := &model.Document{
		APIVersion: model.APIVersion,
		Kind:       model.Kind,
		Metadata:   model.Metadata{Name: "lab-sink"},
	}
	if err := Check(doc, ""); err != nil {
		t.Fatal(err)
	}
	if len(doc.Spec.Admission.AllowClientCIDRs) != 2 {
		t.Fatalf("defaults = %v", doc.Spec.Admission.AllowClientCIDRs)
	}
}

func TestTLSEnabled(t *testing.T) {
	doc := &model.Document{
		APIVersion: model.APIVersion,
		Kind:       model.Kind,
		Metadata:   model.Metadata{Name: "lab-sink"},
	}
	doc.Spec.Listeners.TLS.Enabled = true
	if err := Check(doc, ""); !domainerr.Is(err, domainerr.TLSUnsupported) {
		t.Fatalf("got %v", err)
	}
}

func TestShortTokenWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tok")
	if err := os.WriteFile(p, []byte("short\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := &model.Document{
		APIVersion: model.APIVersion,
		Kind:       model.Kind,
		Metadata:   model.Metadata{Name: "lab-sink"},
		Spec: model.Spec{
			Auth: model.Auth{
				Tokens: []model.AuthToken{{ID: "op", Role: "administrator", SecretFile: p}},
			},
		},
	}
	err := Check(doc, dir)
	if !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "shorter") {
		t.Fatalf("detail %v", err)
	}
}

func TestCompileMissingTokenWhenManagementBound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec:
  listeners:
    management:
      address: "127.0.0.1:0"
  auth:
    tokens:
      - id: op
        secretFile: missing.token
`), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Compile(doc, Options{ConfigDir: dir})
	if !domainerr.Is(err, domainerr.ValidationFailed) {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "secretFile") {
		t.Fatalf("detail %v", err)
	}

	doc2, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(doc2, Options{ConfigDir: dir, ManagementListen: "off"}); err != nil {
		t.Fatal(err)
	}
}

func TestCompileListenOverrides(t *testing.T) {
	doc := &model.Document{
		APIVersion: model.APIVersion,
		Kind:       model.Kind,
		Metadata:   model.Metadata{Name: "lab-sink"},
	}
	snap, err := Compile(doc, Options{
		UDPListen:        "127.0.0.1:0",
		TCPListen:        "127.0.0.1:0",
		ManagementListen: "off",
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Document.Spec.Listeners.UDP.Address != "127.0.0.1:0" {
		t.Fatalf("udp %q", snap.Document.Spec.Listeners.UDP.Address)
	}
	if snap.Document.Spec.Listeners.Management.Address != "" {
		t.Fatalf("management %q", snap.Document.Spec.Listeners.Management.Address)
	}
	if !strings.HasPrefix(snap.Revision, "sha256:") {
		t.Fatalf("revision %q", snap.Revision)
	}
}

func TestRevisionFormat(t *testing.T) {
	rev := Revision([]byte("hello"))
	if !strings.HasPrefix(rev, "sha256:") {
		t.Fatalf("%q", rev)
	}
	hex := strings.TrimPrefix(rev, "sha256:")
	if len(hex) != 64 || strings.ToLower(hex) != hex {
		t.Fatalf("hex %q", hex)
	}
}
