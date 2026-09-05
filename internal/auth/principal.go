package auth

import (
	"context"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

// Credential classes recorded on the principal and audit actor.
const (
	ClassToken   = "token"
	ClassSession = "session"
)

type principalKey struct{}

// Principal is the non-secret actor after authentication.
type Principal struct {
	ID     string
	Class  string
	Role   string
	Scopes []string
}

// HasScope reports whether p grants want. Scopes are exact; administrator
// is expanded at compile time rather than implied at check time.
func (p Principal) HasScope(want string) bool {
	if want == "" {
		return true
	}
	for _, s := range p.Scopes {
		if s == want {
			return true
		}
	}
	return false
}

// Authorize reports forbidden when the required scope is missing.
func Authorize(p Principal, scope string) error {
	if p.HasScope(scope) {
		return nil
	}
	return domainerr.New(domainerr.Forbidden, "missing scope "+scope)
}

// ContextWithPrincipal stores p on ctx for REST/MCP adapters.
func ContextWithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext returns the authenticated principal, if any.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
