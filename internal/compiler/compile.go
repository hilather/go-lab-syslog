package compiler

import (
	"bytes"
	"os"
	"strings"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/snapshot"
	"gopkg.in/yaml.v3"
)

// Options are compile-time overlays and secret resolution.
type Options struct {
	ConfigDir        string
	UDPListen        string
	TCPListen        string
	ManagementListen string
}

// Compile clones, applies listen-flag overlays, normalizes, validates, and
// hashes an immutable Snapshot. When management is bound, missing or short
// token files fail closed (C29). --management-listen=off does not require a
// token file.
func Compile(doc *model.Document, opts Options) (*snapshot.Snapshot, error) {
	if doc == nil {
		return nil, domainerr.New(domainerr.ValidationFailed, "document is empty")
	}
	out := CloneDocument(*doc)
	ApplyListenOverrides(&out, opts)
	if err := Check(&out, opts.ConfigDir); err != nil {
		return nil, err
	}
	if ManagementBound(&out, "") {
		if err := requireTokens(&out, opts.ConfigDir); err != nil {
			return nil, err
		}
	}
	canon, err := encodeCanonical(&out)
	if err != nil {
		return nil, err
	}
	return &snapshot.Snapshot{
		Document:  out,
		Canonical: canon,
		Revision:  Revision(canon),
	}, nil
}

// encodeCanonical matches config.EncodeCanonical without importing config
// (config tests import compiler).
func encodeCanonical(doc *model.Document) ([]byte, error) {
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

// ApplyListenOverrides materializes CLI listen flags onto doc. Empty flag
// values leave the YAML field unchanged. "off" clears management.address.
func ApplyListenOverrides(doc *model.Document, opts Options) {
	if doc == nil {
		return
	}
	if v := strings.TrimSpace(opts.UDPListen); v != "" {
		doc.Spec.Listeners.UDP.Address = v
		t := true
		doc.Spec.Listeners.UDP.Enabled = &t
	}
	if v := strings.TrimSpace(opts.TCPListen); v != "" {
		doc.Spec.Listeners.TCP.Address = v
		t := true
		doc.Spec.Listeners.TCP.Enabled = &t
	}
	if v := strings.TrimSpace(opts.ManagementListen); v != "" {
		if strings.EqualFold(v, "off") {
			doc.Spec.Listeners.Management.Address = ""
		} else {
			doc.Spec.Listeners.Management.Address = v
		}
	}
}

// ManagementBound reports whether the compiled document requests a management
// listen address. flag, when non-empty, overrides spec.listeners.management.address
// the same way Compile does (including "off").
func ManagementBound(doc *model.Document, flag string) bool {
	if doc == nil {
		return false
	}
	addr := strings.TrimSpace(flag)
	if addr == "" {
		addr = strings.TrimSpace(doc.Spec.Listeners.Management.Address)
	}
	return addr != "" && !strings.EqualFold(addr, "off")
}

func requireTokens(doc *model.Document, configDir string) error {
	if len(doc.Spec.Auth.Tokens) == 0 {
		return domainerr.New(domainerr.ValidationFailed, "management bind requires at least one usable token")
	}
	for _, tok := range doc.Spec.Auth.Tokens {
		if err := requireTokenFile(tok.SecretFile, configDir); err != nil {
			return err
		}
	}
	return nil
}

func requireTokenFile(path, configDir string) error {
	data, err := readTokenFile(path, configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return domainerr.Newf(domainerr.ValidationFailed, "secretFile %q is required when management is bound", path)
		}
		return domainerr.Newf(domainerr.ValidationFailed, "secretFile %q: %v", path, err)
	}
	return checkTokenBytes(path, data)
}
