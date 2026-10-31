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
	"dockerpilot/internal/audit"
	"dockerpilot/internal/auth"
	"dockerpilot/internal/collector"
	"dockerpilot/internal/dockerclient"
	"dockerpilot/internal/envx"
	"dockerpilot/internal/logcenter"
	"dockerpilot/internal/observability"
	"dockerpilot/internal/org"
	"dockerpilot/internal/resolved"
	"dockerpilot/internal/store"
	"dockerpilot/internal/telegram"

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

	notifier := alerts.NewFromEnv()
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

	server := api.NewServerFromDeps(api.Deps{
		Hosts: pool, Store: st, LogStore: logStore, Auth: authSvc,
		Audit: auditLog, Orgs: orgs, Resolved: resolvedStore, DataDir: dataDir,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("dockpilot backend ouvindo em :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, server.Router))
}
