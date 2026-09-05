package auth

const (
	RoleAdministrator = "administrator"
	RoleReader        = "reader"

	ScopeRead      = "syslog.read"
	ScopeWrite     = "syslog.write"
	ScopeAdmin     = "syslog.admin"
	ScopeAuditRead = "syslog.audit.read"
)

// DefaultScopes is the frozen role → scope set (docs/05, ADR 0005).
func DefaultScopes(role string) []string {
	switch role {
	case RoleReader:
		return []string{ScopeRead}
	case RoleAdministrator:
		return allScopes()
	default:
		return nil
	}
}

func allScopes() []string {
	return []string{ScopeRead, ScopeWrite, ScopeAdmin, ScopeAuditRead}
}

func expandScopes(role string) (string, []string) {
	if role == "" {
		role = RoleAdministrator
	}
	return role, DefaultScopes(role)
}
