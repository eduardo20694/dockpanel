// Package rbac is the permission matrix for Dockpilot organizations.
package rbac

import "strings"

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleAuditor  = "auditor"
	RoleViewer   = "viewer"
)

// Allow reports whether role may call method+path.
// An empty role means authentication is off (single-operator mode) and is allowed.
func Allow(role, method, path string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return true
	}
	method = strings.ToUpper(method)
	read := method == "GET" || method == "HEAD" || method == "OPTIONS" || (method == "POST" && path == "/api/secrets/scan")
	switch role {
	case RoleAdmin:
		return true
	case RoleOperator:
		if read {
			return true
		}
		return !isIdentityAdmin(path)
	case RoleAuditor:
		if !read {
			return false
		}
		return true
	case RoleViewer:
		if !read {
			return false
		}
		if strings.HasPrefix(path, "/api/audit") || strings.HasPrefix(path, "/api/secrets") || strings.HasPrefix(path, "/api/sessions") || strings.HasPrefix(path, "/api/reports") {
			return false
		}
		return true
	default:
		return read && !strings.HasPrefix(path, "/api/audit")
	}
}

func isIdentityAdmin(path string) bool {
	switch {
	case strings.HasPrefix(path, "/api/users"),
		strings.HasPrefix(path, "/api/teams"),
		strings.HasPrefix(path, "/api/orgs"):
		return true
	default:
		return false
	}
}

// CanReadAudit is true for roles that may open the audit log.
func CanReadAudit(role string) bool {
	switch strings.ToLower(role) {
	case RoleAdmin, RoleOperator, RoleAuditor, "":
		return true
	default:
		return false
	}
}
