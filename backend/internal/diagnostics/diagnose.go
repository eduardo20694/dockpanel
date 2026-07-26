// Package diagnostics é o "cérebro" do dockpanel: em vez de só devolver
// dados crus do Docker, ele cruza sinais (exit code, OOMKilled, contagem
// de restart, padrões de erro nos logs) num relatório único que tanto o
// dashboard quanto o servidor MCP conseguem usar. É o que complementa o
// Portainer — ele te mostra o dado, isso aqui te dá a leitura do dado.
package diagnostics

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

type Severity string

const (
	SeverityOK       Severity = "ok"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Diagnosis struct {
	ContainerID   string   `json:"containerId"`
	Name          string   `json:"name"`
	Severity      Severity `json:"severity"`
	State         string   `json:"state"`
	ExitCode      int      `json:"exitCode"`
	OOMKilled     bool     `json:"oomKilled"`
	RestartCount  int      `json:"restartCount"`
	StartedAt     string   `json:"startedAt"`
	FinishedAt    string   `json:"finishedAt"`
	Findings      []string `json:"findings"`      // frases legíveis do que foi encontrado
	ErrorLines    []string `json:"errorLines"`     // linhas de log que bateram nos padrões de erro
	RecentEvents    []string         `json:"recentEvents"`
	RelatedFindings []RelatedFinding `json:"relatedFindings"`
	Recommendation  string           `json:"recommendation"`
}

type Engine struct {
	CLI *client.Client
}

func New(cli *client.Client) *Engine {
	return &Engine{CLI: cli}
}

// Diagnose monta o relatório completo de um container específico.
func (e *Engine) Diagnose(ctx context.Context, id string) (*Diagnosis, error) {
	info, err := e.CLI.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}

	d := &Diagnosis{
		ContainerID:  info.ID,
		Name:         strings.TrimPrefix(info.Name, "/"),
		State:        info.State.Status,
		ExitCode:     info.State.ExitCode,
		OOMKilled:    info.State.OOMKilled,
		RestartCount: info.RestartCount,
		StartedAt:    info.State.StartedAt,
		FinishedAt:   info.State.FinishedAt,
		Findings:     []string{},
		ErrorLines:   []string{},
		RecentEvents:    []string{},
		RelatedFindings: []RelatedFinding{},
	}

	// --- sinais estruturais (o que o Docker já sabe sobre o motivo da parada) ---
	if info.State.OOMKilled {
		d.Findings = append(d.Findings, "processo foi morto por falta de memória (OOM killed) — o container excedeu o limite de RAM configurado (ou o do host)")
	}
	if info.State.ExitCode != 0 && info.State.Status != "running" {
		d.Findings = append(d.Findings, fmt.Sprintf("processo encerrou com exit code %d", info.State.ExitCode))
	}
	if info.RestartCount >= 3 {
		d.Findings = append(d.Findings, fmt.Sprintf("já reiniciou %d vezes — padrão de crash loop", info.RestartCount))
	}
	if info.State.Status == "restarting" {
		d.Findings = append(d.Findings, "está em ciclo de reinício agora mesmo")
	}
	if info.State.Error != "" {
		d.Findings = append(d.Findings, fmt.Sprintf("erro reportado pelo próprio Docker: %s", info.State.Error))
	}

	// --- logs recentes, filtrados pelos padrões de erro ---
	logLines, err := e.tailErrorLogs(ctx, id, 200)
	if err == nil {
		d.ErrorLines = logLines
	}

	// --- eventos recentes do Docker pra esse container (die, oom, health_status) ---
	evs, err := e.recentEvents(ctx, id, 30*time.Minute)
	if err == nil {
		d.RecentEvents = evs
	}

	// --- correlaciona com outros containers (mesma rede / compose) ---
	if related := e.correlateDependencies(ctx, info, d.ErrorLines); len(related) > 0 {
		d.RelatedFindings = related
		for _, r := range related {
			if r.State != "running" && strings.Contains(r.Detail, "PARADA") {
				d.Findings = append(d.Findings, fmt.Sprintf("dependência %s (%s) — %s", r.Name, r.Relation, r.Detail))
			}
		}
	}

	d.Severity = classify(d)
	d.Recommendation = recommend(d)

	return d, nil
}

// tailErrorLogs lê as últimas `tail` linhas de log e devolve só as que
// batem nos padrões de erro conhecidos (evita jogar 300 linhas de log
// saudável na cara de quem só quer saber o que quebrou).
func (e *Engine) tailErrorLogs(ctx context.Context, id string, tail int) ([]string, error) {
	reader, err := e.CLI.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
		Timestamps: false,
	})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var matches []string
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := cleanDockerLogLine(scanner.Text())
		if line == "" {
			continue
		}
		if LogLineLooksLikeError(line) {
			matches = append(matches, line)
		}
	}
	if len(matches) > 20 {
		matches = matches[len(matches)-20:] // as 20 mais recentes bastam
	}
	return matches, nil
}

// cleanDockerLogLine remove os 8 bytes de header do stream multiplexado
// do Docker quando o container não usa TTY.
func cleanDockerLogLine(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(cleaned)
}

func (e *Engine) recentEvents(ctx context.Context, id string, window time.Duration) ([]string, error) {
	since := time.Now().Add(-window)
	msgs, errs := e.CLI.Events(ctx, types.EventsOptions{
		Since: since.Format(time.RFC3339),
	})

	var out []string
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case ev := <-msgs:
			if eventMatchesContainer(ev, id) {
				ts := time.Unix(ev.Time, 0).Format(time.RFC3339)
				out = append(out, fmt.Sprintf("%s: %s", ev.Action, ts))
			}
		case err := <-errs:
			if err != nil && err != io.EOF {
				return out, nil
			}
			return out, nil
		case <-timeout:
			return out, nil
		}
	}
}

func eventMatchesContainer(ev events.Message, id string) bool {
	cid := ev.Actor.ID
	return cid == id ||
		strings.HasPrefix(id, cid) ||
		strings.HasPrefix(cid, id)
}

func classify(d *Diagnosis) Severity {
	if d.OOMKilled || (d.ExitCode != 0 && d.State != "running") || d.RestartCount >= 5 {
		return SeverityCritical
	}
	if len(d.ErrorLines) > 0 || d.RestartCount > 0 || d.State == "restarting" {
		return SeverityWarning
	}
	return SeverityOK
}

func recommend(d *Diagnosis) string {
	switch {
	case d.OOMKilled:
		return "aumente o limite de memória do container (mem_limit no compose) ou investigue vazamento de memória na aplicação"
	case d.RestartCount >= 5:
		return "provável crash loop — veja as linhas de erro abaixo e o log completo antes de reiniciar de novo; reiniciar sem entender a causa só adia o problema"
	case d.ExitCode == 137:
		return "exit code 137 costuma ser SIGKILL — geralmente OOM ou um `docker stop` forçado; confira memória disponível no host"
	case d.ExitCode == 1 && len(d.ErrorLines) > 0:
		return "erro de aplicação — veja as linhas de log filtradas abaixo, é provável que a causa esteja ali"
	case d.ExitCode != 0 && d.State != "running":
		return fmt.Sprintf("processo encerrou com exit code %d — confira os logs completos do container", d.ExitCode)
	case len(d.ErrorLines) > 0:
		return "container rodando, mas com mensagens de erro no log — vale investigar mesmo sem estar caindo"
	default:
		return "nenhum problema óbvio encontrado"
	}
}
