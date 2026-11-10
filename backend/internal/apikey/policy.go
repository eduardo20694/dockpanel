package apikey

import (
	"strings"

	"dockerpilot/internal/rbac"
)

// mutatingTools alteram o host ou o diretório.
var mutatingTools = map[string]string{
	"container_action":    "/api/containers/x/start",
	"remove_resource":     "/api/containers/x",
	"deploy_compose":      "/api/deploy/compose",
	"backup_volume":       "/api/volumes/x/backup",
	"create_stack":        "/api/stacks/apply",
	"update_stack":        "/api/stacks/apply",
	"rollback_stack":      "/api/stacks/rollback",
	"create_user":         "/api/users",
	"create_team":         "/api/teams",
	"add_team_member":     "/api/teams/x/members",
	"resolve_problem":     "/api/system/problems/resolve",
	"log_retention":       "/api/logs/incidents/retention",
	"docker_proxy":        "/api/proxy/docker",
	"kubernetes_proxy":    "/api/proxy/kubernetes",
	"gitops_configure":    "/api/gitops",
	"gitops_sync":         "/api/gitops/sync",
	"gitops_approve":      "/api/gitops/approve",
	"secrets_put":         "/api/secrets",
	"secrets_delete":      "/api/secrets/x",
	"apply_image_update":  "/api/updates/apply",
	"image_update_policy": "/api/updates/policy",
	"create_guest_access": "/api/sessions",
	"revoke_guest_access": "/api/sessions/x",
	"deploy_template":     "/api/templates/x/deploy",
	"render_report":       "/api/reports",
	"container_exec":       "/api/containers/x/exec",
	"swarm_scale":          "/api/swarm/services/x/scale",
	"swarm_update":         "/api/swarm/services/x/update",
	"swarm_rollback":       "/api/swarm/services/x/rollback",
	"swarm_secret_create":  "/api/swarm/secrets",
	"swarm_secret_delete":  "/api/swarm/secrets/x",
	"swarm_config_create":  "/api/swarm/configs",
	"swarm_config_delete":  "/api/swarm/configs/x",
	"swarm_stack_deploy":   "/api/swarm/stacks",
	"swarm_stack_remove":   "/api/swarm/stacks/x",
	"k8s_scale":            "/api/k8s/scale",
	"k8s_rollout":          "/api/k8s/rollout",
	"k8s_undo":             "/api/k8s/undo",
	"k8s_apply":            "/api/k8s/apply",
	"k8s_helm":             "/api/k8s/helm",
	"k8s_pod_exec":         "/api/k8s/pods/x/exec",
	"k8s_pod_restart":      "/api/k8s/pods/x/restart",
}

var stackTools = map[string]bool{
	"gitops_configure": true, "gitops_sync": true, "gitops_approve": true,
	"secrets_put": true, "secrets_delete": true, "secrets_list": true, "secrets_scan": true,
	"deploy_template": true,
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
	switch name {
	case "list_audit", "session_timeline":
		path = "/api/audit"
	case "secrets_list", "secrets_scan":
		path = "/api/secrets"
	case "gitops_status", "gitops_diff":
		path = "/api/gitops"
	case "image_updates":
		path = "/api/updates"
	case "list_templates":
		path = "/api/templates"
	}
	if !rbac.Allow(p.Role, method, path) {
		return ErrForbidden
	}
	return nil
}

// AllowsCall enforces host and stack scope. Unscoped permanent keys are unchanged.
func (p Principal) AllowsCall(tool, host, stack string) error {
	if !p.Guest && len(p.Hosts) == 0 && len(p.Stacks) == 0 {
		return nil
	}
	if len(p.Hosts) > 0 && !has(p.Hosts, host) {
		return ErrForbidden
	}
	if len(p.Stacks) > 0 && (stack != "" || stackTools[tool]) && !has(p.Stacks, stack) {
		return ErrForbidden
	}
	return nil
}

func has(list []string, v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func IsMutating(name string) bool {
	_, ok := mutatingTools[name]
	return ok
}
