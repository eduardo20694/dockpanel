package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"dockerpilot/internal/alerts"
	"dockerpilot/internal/api"
	"dockerpilot/internal/apikey"
	"dockerpilot/internal/audit"
	"dockerpilot/internal/auth"
	"dockerpilot/internal/collector"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/envx"
	"dockerpilot/internal/gitops"
	"dockerpilot/internal/kube"
	"dockerpilot/internal/logcenter"
	"dockerpilot/internal/observability"
	"dockerpilot/internal/org"
	"dockerpilot/internal/resolved"
	"dockerpilot/internal/secrets"
	"dockerpilot/internal/store"
	"dockerpilot/internal/telegram"
	"dockerpilot/internal/updates"

	"github.com/joho/godotenv"
)

func loadEnv() {
	candidates := []string{
		filepath.Join("..", ".env"),
		".env",
	}
	if root := envx.Get("ROOT"); root != "" {
		candidates = append([]string{filepath.Join(root, ".env")}, candidates...)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := godotenv.Load(p); err != nil {
			log.Printf("aviso: erro ao ler %s: %v", p, err)
			continue
		}
		log.Printf("config: .env carregado (%s)", p)
		return
	}
	log.Printf("config: nenhum .env encontrado — use variáveis do sistema ou crie ../.env")
}

func main() {
	loadEnv()
	observability.Init()
	defer observability.Flush()

	pool, err := dockerclient.NewPoolFromEnv()
	if err != nil {
		log.Fatalf("erro ao conectar hosts Docker: %v", err)
	}

	st, err := store.Open("")
	if err != nil {
		log.Printf("aviso: histórico desabilitado: %v", err)
	}

	for _, h := range pool.List() {
		cli, _ := pool.Get(h.ID)
		if err := cli.Ping(context.Background()); err != nil {
			log.Printf("aviso: host %q (%s) não respondeu ping: %v", h.ID, h.Label, err)
		} else {
			log.Printf("host Docker ok: %s (%s)", h.Label, h.ID)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	authSvc, err := auth.NewService()
	if err != nil {
		log.Fatalf("auth: %v", err)
	}
	if authSvc.Enabled {
		log.Printf("auth: login ativo (admin env)")
	} else {
		log.Printf("auth: desabilitado (defina DOCKERPILOT_ADMIN_EMAIL e DOCKERPILOT_ADMIN_PASSWORD)")
	}

	dataDir := envx.DataDir()
	var auditLog *audit.Log
	if al, err := audit.Open(dataDir); err != nil {
		log.Printf("aviso: auditoria desabilitada: %v", err)
	} else {
		auditLog = al
	}
	var orgs *org.Store
	if og, err := org.Open(dataDir); err != nil {
		log.Printf("aviso: organizações desabilitadas: %v", err)
	} else {
		orgs = og
	}
	var resolvedStore *resolved.Store
	if rs, err := resolved.Open(dataDir); err != nil {
		log.Printf("aviso: problemas resolvidos desabilitados: %v", err)
	} else {
		resolvedStore = rs
	}
	var keys *apikey.Store
	if ks, err := apikey.Open(dataDir); err != nil {
		log.Printf("aviso: chaves MCP desabilitadas: %v", err)
	} else {
		keys = ks
	}
	var gitStore *gitops.Store
	if gs, err := gitops.Open(dataDir); err != nil {
		log.Printf("aviso: gitops desabilitado: %v", err)
	} else {
		gitStore = gs
		gitStore.Start(ctx, auditLog)
	}
	var secretStore *secrets.Store
	if ss, err := secrets.Open(dataDir); err != nil {
		log.Printf("aviso: cofre desabilitado: %v", err)
	} else {
		secretStore = ss
	}
	var updateSvc *updates.Service
	if us, err := updates.Open(dataDir); err != nil {
		log.Printf("aviso: atualizações desabilitadas: %v", err)
	} else {
		updateSvc = us
		if c, err := pool.Get(pool.DefaultID()); err == nil && c != nil {
			updateSvc.Engine = updates.NewDockerEngine(c.CLI)
		}
		updateSvc.Start(ctx, auditLog)
	}
	notifier := alerts.NewFromEnv()
	if updateSvc != nil {
		updateSvc.Notify = func(title, body string) { _ = notifier.SendSeverity("warning", title, body, nil) }
	}
	alerts.NewScanner(pool, notifier, st).Start(ctx)
	telegram.NewFromEnv(pool, notifier).Start(ctx)
	collector.New(pool, st).Start(ctx)

	var logStore *logcenter.Store
	if ls, err := logcenter.Open(""); err != nil {
		log.Printf("aviso: log center desabilitado: %v", err)
	} else {
		logStore = ls
		logcenter.NewCollector(pool, logStore).Start(ctx)
		defer logStore.Close()
	}

	var kubeSources []kube.Source
	for _, h := range pool.List() {
		if h.Kind == "kubernetes" || h.Kind == "k8s" {
			kubeSources = append(kubeSources, kube.Source{ID: h.ID, Label: h.Label, Kubeconfig: h.Kubeconfig, Context: h.Context})
		}
	}
	kubeSources = append(kubeSources, kube.SourcesFromEnv()...)
	kubeSet := kube.Open(kubeSources)

	server := api.NewServerFromDeps(api.Deps{
		Hosts: pool, Store: st, LogStore: logStore, Auth: authSvc,
		Audit: auditLog, Orgs: orgs, Resolved: resolvedStore, DataDir: dataDir,
		Keys: keys, GitOps: gitStore, Secrets: secretStore, Updates: updateSvc, Kube: kubeSet,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("dockpilot backend ouvindo em :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, server.Router))
}
