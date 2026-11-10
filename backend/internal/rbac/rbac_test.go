package rbac

import "testing"

func TestMatrix(t *testing.T) {
	cases := []struct {
		role, method, path string
		ok                 bool
	}{
		{"viewer", "GET", "/api/containers/", true},
		{"viewer", "POST", "/api/containers/x/start", false},
		{"viewer", "GET", "/api/audit", false},
		{"auditor", "GET", "/api/audit", true},
		{"auditor", "DELETE", "/api/volumes/x", false},
		{"operator", "POST", "/api/containers/x/restart", true},
		{"operator", "POST", "/api/users", false},
		{"operator", "GET", "/api/users", true},
		{"admin", "DELETE", "/api/teams/t", true},
		{"", "POST", "/api/users", true},
	}
	for _, c := range cases {
		if got := Allow(c.role, c.method, c.path); got != c.ok {
			t.Errorf("%s %s %s: got %v want %v", c.role, c.method, c.path, got, c.ok)
		}
	}
}
