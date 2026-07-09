package api

import (
	"encoding/json"
	"net/http"

	"dockpanel/internal/auth"
	"dockpanel/internal/dockerclient"
	"dockpanel/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Server struct {
	Hosts  *dockerclient.Pool
	Store  *store.Store
	Auth   *auth.Service
	Router *chi.Mux
}

func NewServer(hosts *dockerclient.Pool, st *store.Store, authSvc *auth.Service) *Server {
	s := &Server{Hosts: hosts, Store: st, Auth: authSvc, Router: chi.NewRouter()}
	s.routes()
	return s
}

func (s *Server) routes() {
	r := s.Router
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:9090"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Dockpanel-Host"},
		AllowCredentials: true,
	}))
	r.Use(s.authMiddleware)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})

	r.Get("/api/auth/config", s.authConfig)
	r.Post("/api/auth/login", s.authLogin)
	r.Post("/api/auth/logout", s.authLogout)
	r.Get("/api/auth/me", s.authMe)

	r.Get("/api/hosts", s.listHosts)

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
	r.Get("/api/history/alerts", s.listAlertHistory)

	r.Get("/api/deploy/presets", s.deployPresets)
	r.Post("/api/deploy/compose", s.deployCompose)
	r.Get("/api/deploy/compose/status", s.deployComposeStatus)

	r.Route("/api/containers", func(r chi.Router) {
		r.Get("/", s.listContainers)
		r.Get("/stats/live", s.liveContainerStats)
		r.Get("/{id}", s.inspectContainer)
		r.Get("/{id}/logs", s.logsContainer)
		r.Get("/{id}/diagnose", s.diagnoseContainer)
		r.Get("/{id}/investigate", s.investigateContainer)
		r.Get("/{id}/history", s.containerHistory)
		r.Post("/{id}/start", s.startContainer)
		r.Post("/{id}/stop", s.stopContainer)
		r.Post("/{id}/restart", s.restartContainer)
		r.Delete("/{id}", s.removeContainer)
		r.Post("/{id}/exec", s.execCreate)
		r.Get("/{id}/stats/ws", s.wsStats)
		r.Get("/{id}/terminal/ws", s.wsTerminal)
	})

	r.Route("/api/images", func(r chi.Router) {
		r.Get("/", s.listImages)
		r.Post("/pull", s.pullImage)
		r.Post("/{id}/scan", s.scanImage)
		r.Delete("/{id}", s.removeImage)
	})

	r.Route("/api/volumes", func(r chi.Router) {
		r.Get("/", s.listVolumes)
		r.Post("/{name}/backup", s.backupVolume)
		r.Delete("/{name}", s.removeVolume)
	})

	r.Route("/api/networks", func(r chi.Router) {
		r.Get("/", s.listNetworks)
		r.Get("/{id}", s.inspectNetwork)
		r.Delete("/{id}", s.removeNetwork)
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
