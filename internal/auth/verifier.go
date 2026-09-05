package auth

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

const realmBearer = `Bearer realm="labsyslog"`

// Verifier is the process-local token index. There is no HTTP Basic.
// Adapters (REST and later MCP) share this type.
type Verifier struct {
	mode   string
	tokens []storedToken
}

type storedToken struct {
	id     string
	role   string
	scopes []string
	digest [sha256.Size]byte
}

// Request is one authentication attempt. Adapters fill it from the HTTP
// request; X-Forwarded-For is never consulted.
type Request struct {
	Authorization string
	RemoteAddr    string
}

// FromSpec compiles spec.auth. Missing secret files are skipped (validate
// semantics). Existing short files fail closed.
func FromSpec(spec model.Auth, configDir string) (*Verifier, error) {
	mode := strings.TrimSpace(spec.Mode)
	if mode == "" {
		mode = "bearer"
	}
	if mode != "bearer" {
		return nil, domainerr.Newf(domainerr.ValidationFailed, "spec.auth.mode must be bearer, got %q", mode)
	}

	seenID := map[string]struct{}{}
	seenDigest := map[[sha256.Size]byte]string{}
	tokens := make([]storedToken, 0, len(spec.Tokens))
	for _, tok := range spec.Tokens {
		id := strings.TrimSpace(tok.ID)
		if id == "" {
			return nil, domainerr.New(domainerr.ValidationFailed, "token id is required")
		}
		if _, ok := seenID[id]; ok {
			return nil, domainerr.Newf(domainerr.ValidationFailed, "duplicate token id %q", id)
		}
		raw, err := readSecretFile(tok.SecretFile, configDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, domainerr.Newf(domainerr.ValidationFailed, "secretFile %q: %v", tok.SecretFile, err)
		}
		if len(raw) < MinTokenBytes {
			zero(raw)
			return nil, domainerr.Newf(domainerr.ValidationFailed, "secretFile %q trimmed contents are shorter than %d bytes", tok.SecretFile, MinTokenBytes)
		}
		d := DigestSecret(raw)
		zero(raw)
		if other, ok := seenDigest[d]; ok {
			return nil, domainerr.Newf(domainerr.ValidationFailed, "token value matches %s", other)
		}
		role, scopes := expandScopes(tok.Role)
		if len(scopes) == 0 {
			return nil, domainerr.Newf(domainerr.ValidationFailed, "unknown role %q", tok.Role)
		}
		seenID[id] = struct{}{}
		seenDigest[d] = id
		tokens = append(tokens, storedToken{id: id, role: role, scopes: scopes, digest: d})
	}
	return &Verifier{mode: mode, tokens: tokens}, nil
}

// Static builds a bearer verifier from an in-memory secret (tests).
func Static(secret, id, role string) *Verifier {
	if id == "" {
		id = "operator"
	}
	r, scopes := expandScopes(role)
	return &Verifier{
		mode: "bearer",
		tokens: []storedToken{{
			id:     id,
			role:   r,
			scopes: scopes,
			digest: DigestSecret([]byte(secret)),
		}},
	}
}

// Mode is the compiled auth mode.
func (v *Verifier) Mode() string {
	if v == nil {
		return ""
	}
	return v.mode
}

// TokenCount is the number of compiled bearer principals.
func (v *Verifier) TokenCount() int {
	if v == nil {
		return 0
	}
	return len(v.tokens)
}

// WWWAuthenticate is the 401 challenge. There is no Basic.
func WWWAuthenticate() string {
	return realmBearer
}

// Authenticate verifies Authorization. A missing or non-Bearer header is
// unauthenticated. Loopback still requires a token (ADR 0005).
func (v *Verifier) Authenticate(in Request) (Principal, error) {
	if v == nil {
		return Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
	}
	h := strings.TrimSpace(in.Authorization)
	if h == "" {
		return Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
	}
	scheme, rest, ok := strings.Cut(h, " ")
	if !ok {
		return Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
	}
	rest = strings.TrimSpace(rest)
	if !strings.EqualFold(scheme, "Bearer") {
		return Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
	}
	return v.lookupBearer(rest)
}

// AuthenticateBearer looks up a raw token secret (mcp-stdio --token-file).
func (v *Verifier) AuthenticateBearer(secret string) (Principal, error) {
	if v == nil {
		return Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
	}
	return v.lookupBearer(strings.TrimSpace(secret))
}

func (v *Verifier) lookupBearer(secret string) (Principal, error) {
	if secret == "" || strings.ContainsAny(secret, " \t") {
		return Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
	}
	digest := DigestSecret([]byte(secret))
	found := 0
	idx := 0
	for i, t := range v.tokens {
		eq := 0
		if EqualDigest(t.digest, digest) {
			eq = 1
		}
		mask := eq
		idx = idx*(1-mask) + i*mask
		found += eq
	}
	if found != 1 {
		return Principal{}, domainerr.New(domainerr.Unauthorized, "authentication required")
	}
	return principalOf(v.tokens[idx]), nil
}

func principalOf(t storedToken) Principal {
	return Principal{
		ID:     t.id,
		Class:  ClassToken,
		Role:   t.role,
		Scopes: append([]string(nil), t.scopes...),
	}
}

func readSecretFile(path, configDir string) ([]byte, error) {
	resolved := path
	if !filepath.IsAbs(path) && configDir != "" {
		resolved = filepath.Join(configDir, path)
	}
	b, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSpace(b), nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
