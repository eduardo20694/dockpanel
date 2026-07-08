package alerts

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"dockpanel/internal/diagnostics"
	"dockpanel/internal/dockerclient"
	"dockpanel/internal/store"
)

type Scanner struct {
	Pool     *dockerclient.Pool
	Notifier *Notifier
	Store    *store.Store
	Interval time.Duration
	mu       sync.Mutex
	sent     map[string]time.Time // key hostID:containerID
	cooldown time.Duration
}

func NewScanner(pool *dockerclient.Pool, n *Notifier, st *store.Store) *Scanner {
	return &Scanner{
		Pool:     pool,
		Notifier: n,
		Store:    st,
		Interval: 2 * time.Minute,
		sent:     make(map[string]time.Time),
		cooldown: 30 * time.Minute,
	}
}

func (s *Scanner) Start(ctx context.Context) {
	if s.Notifier == nil || !s.Notifier.Enabled() {
		log.Println("alertas: desabilitado (configure ALERT_TELEGRAM_* ou ALERT_DISCORD_WEBHOOK)")
		return
	}
	log.Printf("alertas: scanner ativo a cada %s", s.Interval)
	t := time.NewTicker(s.Interval)
	go func() {
		s.runOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
				s.runOnce(ctx)
			}
		}
	}()
}

func (s *Scanner) runOnce(ctx context.Context) {
	for _, h := range s.Pool.List() {
		cli, err := s.Pool.Get(h.ID)
		if err != nil {
			continue
		}
		eng := diagnostics.New(cli.CLI)
		problems, err := eng.ScanProblems(ctx)
		if err != nil {
			log.Printf("alertas [%s]: %v", h.ID, err)
			continue
		}
		for _, p := range problems {
			if p.Severity != diagnostics.SeverityCritical {
				continue
			}
			key := h.ID + ":" + p.ContainerID
			if s.recentlySent(key) {
				continue
			}
			title := fmt.Sprintf("dockpanel · %s · %s", h.Label, p.Name)
			body := fmt.Sprintf("Host: %s\nMotivo: %s\nEstado: %s · exit %d · restarts %d\nContainer: %s",
				h.Label, p.Reason, p.State, p.ExitCode, p.RestartCount, p.ContainerID[:12])
			if err := s.Notifier.SendCritical(title, body); err != nil {
				log.Printf("alertas: falha ao enviar: %v", err)
				continue
			}
			if s.Store != nil {
				_ = s.Store.RecordAlert(h.ID, p.ContainerID, p.Name, string(p.Severity), title, body)
			}
			s.markSent(key)
			log.Printf("alertas: enviado critical para %s (%s)", p.Name, h.Label)
		}
	}
}

func (s *Scanner) recentlySent(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.sent[key]
	return ok && time.Since(t) < s.cooldown
}

func (s *Scanner) markSent(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent[key] = time.Now()
}
