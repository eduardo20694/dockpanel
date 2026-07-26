package alerts

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"dockpanel/internal/clock"
	"dockpanel/internal/diagnostics"
	"dockpanel/internal/dockerclient"
	"dockpanel/internal/metrics"
	"dockpanel/internal/store"
	"dockpanel/internal/tgmsg"
)

type Scanner struct {
	Pool     *dockerclient.Pool
	Notifier *Notifier
	Store    *store.Store
	Interval time.Duration
	mu       sync.Mutex
	sent     map[string]time.Time
	cooldown time.Duration

	digestHour    int
	lastDigestDay string
}

func NewScanner(pool *dockerclient.Pool, n *Notifier, st *store.Store) *Scanner {
	hour := 9
	if v := strings.TrimSpace(os.Getenv("ALERT_DAILY_HOUR")); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h >= 0 && h <= 23 {
			hour = h
		}
	}
	return &Scanner{
		Pool:       pool,
		Notifier:   n,
		Store:      st,
		Interval:   2 * time.Minute,
		sent:       make(map[string]time.Time),
		cooldown:   30 * time.Minute,
		digestHour: hour,
	}
}

func (s *Scanner) Start(ctx context.Context) {
	if s.Notifier == nil || !s.Notifier.Enabled() {
		log.Println("alertas: histórico local ativo (sem canais externos — opcional: ALERT_*)")
	} else {
		log.Printf("alertas: scanner ativo a cada %s (critical+warning) · digest diário ~%02d:00 (%s)",
			s.Interval, s.digestHour, clock.Location())
	}
	t := time.NewTicker(s.Interval)
	go func() {
		s.runOnce(ctx)
		s.maybeDailyDigest(ctx)
		for {
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
				s.runOnce(ctx)
				s.maybeDailyDigest(ctx)
			}
		}
	}()
}

func (s *Scanner) runOnce(ctx context.Context) {
	for _, h := range s.Pool.List() {
		cli, err := s.Pool.Get(h.ID)
		if err != nil {
			s.evalServerOffline(ctx, h)
			continue
		}
		if err := cli.Ping(ctx); err != nil {
			s.evalServerOffline(ctx, h)
			continue
		}
		eng := diagnostics.New(cli.CLI)
		problems, err := eng.ScanProblems(ctx)
		if err != nil {
			log.Printf("alertas [%s]: %v", h.ID, err)
			continue
		}
		for _, p := range problems {
			title := p.Name
			bodyPlain := fmt.Sprintf("Host: %s\nMotivo: %s\nEstado: %s · exit %d · restarts %d\nContainer: %s",
				h.Label, p.Reason, p.State, p.ExitCode, p.RestartCount, shortID(p.ContainerID))
			html := tgmsg.Alert(string(p.Severity), title, h.Label, p.Reason, p.State, p.ExitCode, p.RestartCount, p.ContainerID)
			s.fireHTML(h, p.ContainerID, p.Name, string(p.Severity),
				fmt.Sprintf("Dockwatch · %s · %s", h.Label, p.Name),
				bodyPlain, html,
				"problem:"+string(p.Severity)+":"+p.Reason)
		}
	}
}

func (s *Scanner) evalServerOffline(_ context.Context, h dockerclient.HostConfig) {
	bodyPlain := fmt.Sprintf("Host %s (%s) não respondeu ping Docker.", h.Label, h.ID)
	html := tgmsg.HostOffline(h.Label, h.ID)
	s.fireHTML(h, "", h.Label, "critical",
		fmt.Sprintf("Dockwatch · %s offline", h.Label),
		bodyPlain, html, "server_offline")
}

func (s *Scanner) fireHTML(h dockerclient.HostConfig, containerID, name, severity, title, bodyPlain, html, ruleKind string) {
	key := h.ID + ":" + containerID + ":" + ruleKind
	if s.recentlySent(key) {
		return
	}
	if s.Notifier != nil && s.Notifier.Enabled() {
		if s.Notifier.TelegramConfigured() {
			if err := s.Notifier.SendTelegramHTML(html); err != nil {
				log.Printf("alertas: falha telegram: %v", err)
			}
		}
		if err := s.Notifier.SendSeverity(severity, title, bodyPlain, []string{"discord", "email", "whatsapp"}); err != nil {
			log.Printf("alertas: falha canais: %v", err)
		}
	}
	if s.Store != nil {
		_ = s.Store.RecordAlert(h.ID, containerID, name, severity, title, bodyPlain)
	}
	s.markSent(key)
	log.Printf("alertas: %s (%s)", title, h.Label)
}

func (s *Scanner) maybeDailyDigest(ctx context.Context) {
	if s.Notifier == nil || !s.Notifier.Enabled() {
		return
	}
	now := clock.Now()
	day := now.Format("2006-01-02")
	s.mu.Lock()
	already := s.lastDigestDay == day
	s.mu.Unlock()
	if already || now.Hour() < s.digestHour {
		return
	}

	html, plain, err := s.buildDailyDigest(ctx, now)
	if err != nil {
		log.Printf("alertas: digest diário falhou: %v", err)
		return
	}
	title := fmt.Sprintf("Dockwatch · resumo diário · %s", now.Format("02/01"))
	if s.Notifier.TelegramConfigured() {
		if err := s.Notifier.SendTelegramHTML(html); err != nil {
			log.Printf("alertas: falha ao enviar digest: %v", err)
			return
		}
	}
	_ = s.Notifier.SendSeverity("daily", title, plain, []string{"discord", "email", "whatsapp"})
	if s.Store != nil {
		_ = s.Store.RecordAlert("system", "", "daily", "daily", title, plain)
	}
	s.mu.Lock()
	s.lastDigestDay = day
	s.mu.Unlock()
	log.Printf("alertas: digest diário enviado (%s)", day)
}

func (s *Scanner) buildDailyDigest(ctx context.Context, now time.Time) (html, plain string, err error) {
	var problems []tgmsg.ProblemItem
	for _, h := range s.Pool.List() {
		cli, getErr := s.Pool.Get(h.ID)
		if getErr != nil {
			problems = append(problems, tgmsg.ProblemItem{
				Severity: "critical", Name: h.Label, Reason: "host inacessível", Host: h.Label,
			})
			continue
		}
		eng := diagnostics.New(cli.CLI)
		list, scanErr := eng.ScanProblems(ctx)
		if scanErr != nil {
			problems = append(problems, tgmsg.ProblemItem{
				Severity: "warning", Name: h.Label, Reason: scanErr.Error(), Host: h.Label,
			})
			continue
		}
		for _, p := range list {
			problems = append(problems, tgmsg.ProblemItem{
				Severity: string(p.Severity), Name: p.Name, Reason: p.Reason, Host: h.Label,
			})
		}
	}

	stats, snapErr := metrics.Snapshot(ctx, s.Pool)
	running := 0
	var top []tgmsg.MetricItem
	total := 0
	if snapErr != nil {
		plain = "Métricas indisponíveis: " + snapErr.Error()
	} else {
		total = len(stats)
		for _, st := range stats {
			if st.State == "running" {
				running++
				if len(top) < 8 {
					top = append(top, tgmsg.MetricItem{
						Name: st.Name, Host: st.HostLabel, State: st.State,
						CPUPct: st.CPUPct, MemPct: st.MemPct,
						MemHuman: metrics.FormatBytes(st.MemUsage), Running: true,
					})
				}
			}
		}
		plain = fmt.Sprintf("Problemas: %d\nRodando: %d/%d", len(problems), running, total)
	}

	html = tgmsg.Daily(now, problems, total, running, top)
	return html, plain, nil
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

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
