package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"dockerpilot/internal/agent"
	"dockerpilot/internal/apikey"
	"dockerpilot/internal/audit"
	"dockerpilot/internal/auth"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/envx"
	"dockerpilot/internal/gitops"
	"dockerpilot/internal/kube"
	"dockerpilot/internal/logcenter"
	"dockerpilot/internal/notify"
	"dockerpilot/internal/org"
	"dockerpilot/internal/resolved"
	"dockerpilot/internal/secrets"
	"dockerpilot/internal/store"
	"dockerpilot/internal/updates"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Server struct {
	Hosts    *dockerclient.Pool
	Store    *store.Store
	LogStore *logcenter.Store
	Auth     *auth.Service
	Audit    *audit.Log
	Orgs     *org.Store
	Resolved *resolved.Store
	Keys     *apikey.Store
	GitOps   *gitops.Store
	Secrets  *secrets.Store
	Updates  *updates.Service
	Kube     *kube.Set
	Agents   *agent.Hub
	MFA      *auth.MFAStore
	rules    []notify.Rule
	DataDir  string
	Router   *chi.Mux
}

type Deps struct {
	Hosts    *dockerclient.Pool
	Store    *store.Store
	LogStore *logcenter.Store
	Auth     *auth.Service
	Audit    *audit.Log
	Orgs     *org.Store
	Resolved *resolved.Store
	Keys     *apikey.Store
	GitOps   *gitops.Store
	Secrets  *secrets.Store
	Updates  *updates.Service
	Kube     *kube.Set
	Agents   *agent.Hub
	MFA      *auth.MFAStore
	DataDir  string
}

func NewServerFromDeps(d Deps) *Server {
	if d.DataDir == "" {
		d.DataDir = envx.DataDir()
	}
	s := &Server{
		Hosts: d.Hosts, Store: d.Store, LogStore: d.LogStore, Auth: d.Auth,
		Audit: d.Audit, Orgs: d.Orgs, Resolved: d.Resolved, DataDir: d.DataDir,
		Keys: d.Keys, GitOps: d.GitOps, Secrets: d.Secrets, Updates: d.Updates, Kube: d.Kube, Agents: d.Agents, MFA: d.MFA,
		Router: chi.NewRouter(),
	}
	s.routes()
	return s
}

func (s *Server) dataDir() string {
	if s != nil && s.DataDir != "" {
		return s.DataDir
	}
	return envx.DataDir()
}

func (s *Server) routes() {
	r := s.Router
	r.Use(s.requestIDMiddleware)
	r.Use(s.recoverMiddleware)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:9090",
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Dockerpilot-Host", "X-Dockpanel-Host", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
	}))
	r.Use(s.authMiddleware)
	r.Use(s.readonlyMiddleware)
	r.Use(s.rbacMiddleware)
	r.Use(s.auditMiddleware)

	r.Get("/metrics", s.prometheusMetrics)
	r.Get("/api/health", s.health)
	r.Get("/api/health/detailed", s.healthDetailed)
	r.Get("/api/openapi.json", s.openAPISpec)
	r.Post("/api/client-errors", s.clientErrorLog)

	r.Get("/api/auth/config", s.authConfig)
	r.Get("/api/auth/sso", s.authSSOConfig)
	r.Post("/api/auth/sso/exchange", s.authSSOExchange)
	r.Post("/api/auth/login", s.authLogin)
	r.Post("/api/auth/logout", s.authLogout)
	r.Get("/api/auth/me", s.authMe)
	r.Get("/api/auth/identity", s.identityStatus)
	r.Get("/api/observability", s.observabilityStatus)
	r.Get("/api/agents", s.listAgents)
	r.Post("/api/agents", s.enrollAgent)
	r.Post("/api/agents/revoke", s.revokeAgent)
	r.Post("/api/security/sarif", s.sarifExport)
	r.Post("/api/security/cve-policy", s.cvePolicy)
	r.Post("/api/compose/preview", s.composePreview)
	r.Post("/api/compose/render", s.composeRender)
	r.Get("/api/files", s.browseFiles)
	r.Get("/api/notify/rules", s.notifyRules)
	r.Put("/api/notify/rules", s.notifyRules)
	r.Post("/api/notify/preview", s.notifyPreview)

	r.Get("/api/hosts", s.listHosts)
	r.Get("/api/swarm/nodes", s.swarmNodes)
	r.Get("/api/swarm/services", s.swarmServices)
	r.Post("/api/swarm/services/{name}/scale", s.swarmScale)
	r.Post("/api/swarm/services/{name}/update", s.swarmUpdate)
	r.Post("/api/swarm/services/{name}/rollback", s.swarmRollback)
	r.Get("/api/swarm/services/{name}/tasks", s.swarmTasks)
	r.Get("/api/swarm/services/{name}/logs", s.swarmLogs)
	r.Get("/api/swarm/secrets", s.swarmSecrets)
	r.Post("/api/swarm/secrets", s.swarmSecretCreate)
	r.Delete("/api/swarm/secrets/{id}", s.swarmSecretDelete)
	r.Get("/api/swarm/configs", s.swarmConfigs)
	r.Post("/api/swarm/configs", s.swarmConfigCreate)
	r.Delete("/api/swarm/configs/{id}", s.swarmConfigDelete)
	r.Get("/api/swarm/stacks", s.swarmStacks)
	r.Post("/api/swarm/stacks", s.swarmStackDeploy)
	r.Delete("/api/swarm/stacks/{name}", s.swarmStackRemove)
	r.Get("/api/k8s/namespaces", s.k8sNamespaces)
	r.Get("/api/k8s/workloads", s.k8sWorkloads)
	r.Get("/api/k8s/pods", s.k8sPods)
	r.Get("/api/k8s/pods/{namespace}/{name}/logs", s.k8sPodLogs)
	r.Post("/api/k8s/pods/{namespace}/{name}/exec", s.k8sPodExec)
	r.Post("/api/k8s/pods/{namespace}/{name}/restart", s.k8sPodRestart)
	r.Get("/api/k8s/services", s.k8sServices)
	r.Get("/api/k8s/ingresses", s.k8sIngresses)
	r.Get("/api/k8s/configmaps", s.k8sConfigMaps)
	r.Get("/api/k8s/secrets", s.k8sSecrets)
	r.Post("/api/k8s/scale", s.k8sScale)
	r.Post("/api/k8s/rollout", s.k8sRollout)
	r.Post("/api/k8s/undo", s.k8sUndo)
	r.Get("/api/k8s/events", s.k8sEvents)
	r.Get("/api/k8s/broke", s.k8sBroke)
	r.Post("/api/k8s/apply", s.k8sApply)
	r.Post("/api/k8s/helm", s.k8sHelm)
	r.Get("/api/metrics/series", s.metricsSeries)
	r.Get("/api/metrics/host", s.metricsHost)
	r.Get("/api/history/alerts", s.listAlertHistory)

	r.Get("/api/system/info", s.systemInfo)
	r.Get("/api/system/df", s.dfUsage)
	r.Get("/api/system/problems", s.scanProblems)
	r.Get("/api/system/problems/resolved", s.listResolvedProblems)
	r.Post("/api/system/problems/resolve", s.resolveProblem)
	r.Delete("/api/system/problems/resolve", s.unresolveProblem)
	r.Get("/api/system/readonly", s.readonlyStatus)
	r.Get("/api/triage", s.whatBroke)
	r.Get("/api/insights/cost", s.costInsights)
	r.Get("/api/audit", s.listAudit)
	r.Get("/api/orgs", s.listOrgs)
	r.Get("/api/users", s.listUsers)
	r.Post("/api/users", s.createUser)
	r.Get("/api/api-keys", s.listAPIKeys)
	r.Post("/api/api-keys", s.createAPIKey)
	r.Delete("/api/api-keys/{id}", s.revokeAPIKey)
	r.Get("/api/gitops", s.gitopsStatus)
	r.Put("/api/gitops", s.gitopsGlobal)
	r.Put("/api/gitops/stacks/{stack}", s.gitopsUpsert)
	r.Post("/api/gitops/stacks/{stack}/sync", s.gitopsSync)
	r.Post("/api/gitops/stacks/{stack}/approve", s.gitopsApprove)
	r.Get("/api/gitops/stacks/{stack}/diff", s.gitopsDiff)
	r.Post("/api/gitops/webhook", s.gitopsWebhook)
	r.Get("/api/secrets", s.listSecrets)
	r.Put("/api/secrets", s.putSecret)
	r.Delete("/api/secrets/{id}", s.deleteSecret)
	r.Post("/api/secrets/scan", s.scanSecrets)
	r.Get("/api/updates", s.updatesStatus)
	r.Put("/api/updates/policy", s.updatesPolicy)
	r.Post("/api/updates/check", s.updatesCheck)
	r.Post("/api/updates/apply", s.updatesApply)
	r.Get("/api/sessions", s.listSessions)
	r.Post("/api/sessions", s.createSession)
	r.Delete("/api/sessions/{id}", s.revokeSession)
	r.Get("/api/sessions/{id}/timeline", s.sessionTimeline)
	r.Get("/api/templates", s.listTemplates)
	r.Post("/api/templates/{id}/deploy", s.deployTemplate)
	r.Post("/api/reports", s.createReport)
	r.Get("/api/reports/{id}", s.getReport)
	r.Get("/api/reports/{id}/pdf", s.getReportPDF)
	r.Patch("/api/users/{id}", s.updateUser)
	r.Get("/api/teams", s.listTeams)
	r.Post("/api/teams", s.createTeam)
	r.Post("/api/teams/{id}/members", s.addTeamMember)
	r.Get("/api/stacks/revisions", s.listStackRevisions)
	r.Post("/api/stacks/apply", s.applyStack)
	r.Post("/api/stacks/rollback", s.rollbackStack)
	r.Post("/api/proxy/docker", s.dockerProxy)
	r.Post("/api/proxy/kubernetes", s.kubernetesProxy)
	r.Get("/api/system/safe-prune", s.safePruneReport)
	r.Get("/api/system/drift", s.composeDrift)
	r.Get("/api/system/drift/deep", s.deepDrift)
	r.Get("/api/system/security", s.securityAudit)
	r.Get("/api/system/security/all", s.securityAuditAll)
	r.Get("/api/executive", s.executiveSummary)
	r.Get("/api/stacks", s.listStacks)
	r.Get("/api/stacks/all", s.listStacksAll)

	r.Get("/api/deploy/presets", s.deployPresets)
	r.Post("/api/deploy/compose", s.deployComposeAudited)
	r.Get("/api/deploy/compose/status", s.deployComposeStatus)

	r.Route("/api/logs", func(r chi.Router) {
		r.Get("/search", s.searchLogs)
		r.Get("/incidents", s.listLogIncidents)
		r.Get("/retention", s.getLogRetention)
		r.Put("/retention", s.putLogRetention)
		r.Get("/incidents/retention", s.getLogRetention)
		r.Put("/incidents/retention", s.putLogRetention)
	})

	r.Route("/api/containers", func(r chi.Router) {
		r.Get("/", s.listContainers)
		r.Get("/stats/live", s.liveContainerStats)
		r.Get("/{id}", s.inspectContainer)
		r.Get("/{id}/logs", s.logsContainer)
		r.Get("/{id}/diagnose", s.diagnoseContainer)
		r.Get("/{id}/investigate", s.investigateContainer)
		r.Get("/{id}/history", s.containerHistory)
		r.Post("/{id}/start", s.startContainerAudited)
		r.Post("/{id}/stop", s.stopContainerAudited)
		r.Post("/{id}/restart", s.restartContainerAudited)
		r.Delete("/{id}", s.removeContainerAudited)
		r.Post("/{id}/exec", s.execCreate)
		r.Get("/{id}/stats/ws", s.wsStats)
		r.Get("/{id}/terminal/ws", s.wsTerminal)
	})

	r.Route("/api/images", func(r chi.Router) {
		r.Get("/", s.listImages)
		r.Post("/pull", s.pullImage)
		r.Post("/{id}/scan", s.scanImage)
		r.Delete("/{id}", s.removeImageAudited)
	})

	r.Route("/api/volumes", func(r chi.Router) {
		r.Get("/", s.listVolumes)
		r.Post("/{name}/backup", s.backupVolumeAudited)
		r.Delete("/{name}", s.removeVolumeAudited)
	})

	r.Route("/api/networks", func(r chi.Router) {
		r.Get("/", s.listNetworks)
		r.Get("/{id}", s.inspectNetwork)
		r.Delete("/{id}", s.removeNetworkAudited)
	})
}

func (s *Server) deployComposeAudited(w http.ResponseWriter, r *http.Request) {
	s.deployCompose(w, r)
}

func (s *Server) startContainerAudited(w http.ResponseWriter, r *http.Request) {
	s.startContainer(w, r)
}

func (s *Server) stopContainerAudited(w http.ResponseWriter, r *http.Request) {
	s.stopContainer(w, r)
}

func (s *Server) restartContainerAudited(w http.ResponseWriter, r *http.Request) {
	s.restartContainer(w, r)
}

func (s *Server) removeContainerAudited(w http.ResponseWriter, r *http.Request) {
	s.removeContainer(w, r)
}

func (s *Server) removeImageAudited(w http.ResponseWriter, r *http.Request) {
	s.removeImage(w, r)
}

func (s *Server) backupVolumeAudited(w http.ResponseWriter, r *http.Request) {
	s.backupVolume(w, r)
}

func (s *Server) removeVolumeAudited(w http.ResponseWriter, r *http.Request) {
	s.removeVolume(w, r)
}

func (s *Server) removeNetworkAudited(w http.ResponseWriter, r *http.Request) {
	s.removeNetwork(w, r)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if v != nil {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			v = reflect.MakeSlice(rv.Type(), 0, 0).Interface()
		}
	}
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	msg := err.Error()
	status := http.StatusBadGateway
	switch {
	case strings.Contains(msg, "não permitido"), strings.Contains(msg, "permissão"):
		status = http.StatusForbidden
	case strings.Contains(msg, "nenhum host"), strings.Contains(msg, "indisponível"):
		status = http.StatusServiceUnavailable
	case strings.Contains(msg, "não encontrad"):
		status = http.StatusNotFound
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
