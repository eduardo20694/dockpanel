package apikey

import "dockerpilot/internal/rbac"

// mutatingTools alteram o host ou o diretório.
var mutatingTools = map[string]string{
	"container_action": "/api/containers/x/start",
	"remove_resource":  "/api/containers/x",
	"deploy_compose":   "/api/deploy/compose",
	"backup_volume":    "/api/volumes/x/backup",
	"create_stack":     "/api/stacks/apply",
	"update_stack":     "/api/stacks/apply",
	"rollback_stack":   "/api/stacks/rollback",
	"create_user":      "/api/users",
	"create_team":      "/api/teams",
	"add_team_member":  "/api/teams/x/members",
	"resolve_problem":  "/api/system/problems/resolve",
	"log_retention":    "/api/logs/incidents/retention",
	"docker_proxy":     "/api/proxy/docker",
	"kubernetes_proxy": "/api/proxy/kubernetes",
}

// AllowTool aplica o papel gravado na chave. docker_proxy e kubernetes_proxy
// podem ser leitura; o proxy interno ainda recusa métodos de escrita no modo somente leitura.
func AllowTool(p Principal, name string) error {
	path := "/api/containers/"
	method := "GET"
	if pth, ok := mutatingTools[name]; ok && name != "docker_proxy" && name != "kubernetes_proxy" && name != "log_retention" {
		path = pth
		method = "POST"
	}
	if name == "list_audit" {
		path = "/api/audit"
	}
	if !rbac.Allow(p.Role, method, path) {
		return ErrForbidden
	}
	return nil
}

func IsMutating(name string) bool {
	_, ok := mutatingTools[name]
	return ok
}
