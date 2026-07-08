package diagnostics

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// Problem é um resumo leve — usado na varredura geral (dashboard e MCP),
// sem gastar tempo lendo logs de todo mundo. Diagnose() completo só roda
// depois, sob demanda, pro container específico que interessa.
type Problem struct {
	ContainerID  string   `json:"containerId"`
	Name         string   `json:"name"`
	Severity     Severity `json:"severity"`
	State        string   `json:"state"`
	ExitCode     int      `json:"exitCode"`
	OOMKilled    bool     `json:"oomKilled"`
	RestartCount int      `json:"restartCount"`
	Reason       string   `json:"reason"`
}

// ScanProblems varre todos os containers (rodando ou não) e devolve só
// os que têm algum sinal de problema, ordenado do mais grave pro mais leve.
func (e *Engine) ScanProblems(ctx context.Context) ([]Problem, error) {
	list, err := e.CLI.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var problems []Problem
	for _, c := range list {
		info, err := e.CLI.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}

		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		reason := ""
		sev := SeverityOK
		switch {
		case info.State.OOMKilled:
			reason = "morto por falta de memória (OOM)"
			sev = SeverityCritical
		case info.RestartCount >= 5:
			reason = "crash loop (5+ reinícios)"
			sev = SeverityCritical
		case info.State.Status == "exited" && info.State.ExitCode != 0:
			reason = "parou com erro"
			sev = SeverityWarning
		case info.State.Status == "restarting":
			reason = "reiniciando agora"
			sev = SeverityWarning
		case info.RestartCount > 0 && info.RestartCount < 5:
			reason = "já reiniciou ao menos uma vez"
			sev = SeverityWarning
		case info.State.Status == "dead":
			reason = "estado 'dead' — precisa remoção manual"
			sev = SeverityCritical
		default:
			continue // sem sinal de problema, pula
		}

		problems = append(problems, Problem{
			ContainerID:  c.ID,
			Name:         name,
			Severity:     sev,
			State:        info.State.Status,
			ExitCode:     info.State.ExitCode,
			OOMKilled:    info.State.OOMKilled,
			RestartCount: info.RestartCount,
			Reason:       reason,
		})
	}

	// críticos primeiro
	orderedCritical := make([]Problem, 0, len(problems))
	orderedWarning := make([]Problem, 0, len(problems))
	for _, p := range problems {
		if p.Severity == SeverityCritical {
			orderedCritical = append(orderedCritical, p)
		} else {
			orderedWarning = append(orderedWarning, p)
		}
	}
	return append(orderedCritical, orderedWarning...), nil
}
