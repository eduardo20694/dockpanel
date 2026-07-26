package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"dockpanel/internal/auth"
	"dockpanel/internal/dockerclient"
	"dockpanel/internal/logcenter"
	"dockpanel/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Server struct {
	Hosts    *dockerclient.Pool
	Store    *store.Store
	LogStore *logcenter.Store
	Auth     *auth.Service
	Router   *chi.Mux
}

type Deps struct {
	Hosts    *dockerclient.Pool
	Store    *store.Store
	LogStore *logcenter.Store
	Auth     *auth.Service
}

func NewServerFromDeps(d Deps) *Server {
	s := &Server{
		Hosts: d.Hosts, Store: d.Store, LogStore: d.LogStore, Auth: d.Auth,
		Router: chi.NewRouter(),
	}
	s.routes()
	return s
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
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Dockpanel-Host", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
	}))
	r.Use(s.authMiddleware)

	r.Get("/api/health", s.health)
	r.Get("/api/health/detailed", s.healthDetailed)
	r.Get("/api/openapi.json", s.openAPISpec)
	r.Post("/api/client-errors", s.clientErrorLog)

	r.Get("/api/auth/config", s.authConfig)
	r.Post("/api/auth/login", s.authLogin)
	r.Post("/api/auth/logout", s.authLogout)
	r.Get("/api/auth/me", s.authMe)

	r.Get("/api/hosts", s.listHosts)
	r.Get("/api/metrics/series", s.metricsSeries)
	r.Get("/api/metrics/host", s.metricsHost)
	r.Get("/api/history/alerts", s.listAlertHistory)

	r.Get("/api/system/info", s.systemInfo)
	r.Get("/api/system/df", s.dfUsage)
	r.Get("/api/system/problems", s.scanProblems)
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
