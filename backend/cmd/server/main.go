package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"dockpanel/internal/alerts"
	"dockpanel/internal/api"
	"dockpanel/internal/collector"
	"dockpanel/internal/dockerclient"
	"dockpanel/internal/store"
)

func main() {
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

	alerts.NewScanner(pool, alerts.NewFromEnv(), st).Start(ctx)
	collector.New(pool, st).Start(ctx)

	server := api.NewServer(pool, st)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("dockpanel backend ouvindo em :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, server.Router))
}
