package tgmsg

import (
	"fmt"
	"html"
	"strings"
	"time"
)

const (
	sep      = "────────────────"
	maxLen   = 3900
	barFull  = "█"
	barEmpty = "░"
)

// Escape prepares text for Telegram HTML parse_mode.
func Escape(s string) string {
	return html.EscapeString(s)
}

// Truncate keeps Telegram under the 4096 limit.
func Truncate(s string) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n\n… <i>(truncado)</i>"
}

func Header(title string) string {
	return fmt.Sprintf("<b>%s</b>\n", Escape(title))
}

func Subtitle(s string) string {
	return fmt.Sprintf("<i>%s</i>\n", Escape(s))
}

func Divider() string {
	return sep + "\n"
}

func Blank() string {
	return "\n"
}

// Field renders a labeled row with spacing.
func Field(label, value string) string {
	return fmt.Sprintf("<b>%s</b>\n%s\n", Escape(label), Escape(value))
}

// FieldInline keeps label and value on one visual block.
func FieldInline(label, value string) string {
	return fmt.Sprintf("%s  <code>%s</code>\n", Escape(label), Escape(value))
}

func severityLabel(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "Critical"
	case "warning":
		return "Warning"
	default:
		return "Info"
	}
}

func Alert(severity, title, host, reason, state string, exitCode, restarts int, containerID string) string {
	var b strings.Builder
	b.WriteString(Header(severityLabel(severity)))
	b.WriteString(Blank())
	b.WriteString(fmt.Sprintf("<b>%s</b>\n", Escape(title)))
	b.WriteString(Blank())
	b.WriteString(Divider())
	b.WriteString(Blank())
	if host != "" {
		b.WriteString(FieldInline("Host", host))
	}
	if reason != "" {
		b.WriteString(Blank())
		b.WriteString(Field("Motivo", reason))
	}
	if state != "" {
		b.WriteString(FieldInline("Estado", state))
	}
	if exitCode != 0 || restarts > 0 {
		b.WriteString(Blank())
		b.WriteString(FieldInline("Exit", fmt.Sprintf("%d", exitCode)))
		b.WriteString(FieldInline("Restarts", fmt.Sprintf("%d", restarts)))
	}
	if containerID != "" {
		b.WriteString(Blank())
		b.WriteString(FieldInline("Container", short(containerID)))
	}
	return Truncate(strings.TrimRight(b.String(), "\n") + "\n")
}

func HostOffline(hostLabel, hostID string) string {
	var b strings.Builder
	b.WriteString(Header("Host offline"))
	b.WriteString(Blank())
	b.WriteString(fmt.Sprintf("<b>%s</b>\n", Escape(hostLabel)))
	b.WriteString(Blank())
	b.WriteString(Divider())
	b.WriteString(Blank())
	b.WriteString("O Docker não respondeu ao ping.\n")
	b.WriteString(Blank())
	b.WriteString(FieldInline("Host", hostLabel))
	if hostID != "" && hostID != hostLabel {
		b.WriteString(FieldInline("ID", hostID))
	}
	return Truncate(b.String())
}

type ProblemItem struct {
	Severity string
	Name     string
	Reason   string
	Host     string
}

type MetricItem struct {
	Name     string
	Host     string
	State    string
	CPUPct   float64
	MemPct   float64
	MemHuman string
	Running  bool
}

func Metrics(total, running int, items []MetricItem) string {
	var b strings.Builder
	b.WriteString(Header("Métricas"))
	b.WriteString(Blank())
	b.WriteString(fmt.Sprintf("<b>%d</b> containers  ·  <b>%d</b> rodando\n", total, running))
	b.WriteString(Blank())
	b.WriteString(Divider())

	if len(items) == 0 {
		b.WriteString(Blank())
		b.WriteString("<i>Nenhum container encontrado.</i>\n")
		return Truncate(b.String())
	}

	for _, it := range items {
		b.WriteString(Blank())
		if !it.Running {
			b.WriteString(fmt.Sprintf("<b>%s</b>\n", Escape(it.Name)))
			b.WriteString(fmt.Sprintf("     <i>%s</i>  ·  %s\n", Escape(it.State), Escape(it.Host)))
			continue
		}
		b.WriteString(fmt.Sprintf("<b>%s</b>\n", Escape(it.Name)))
		if it.Host != "" {
			b.WriteString(fmt.Sprintf("     <i>%s</i>\n", Escape(it.Host)))
		}
		b.WriteString(Blank())
		b.WriteString(fmt.Sprintf("     CPU   %s  <code>%.1f%%</code>\n", bar(it.CPUPct, 100), it.CPUPct))
		b.WriteString(fmt.Sprintf("     RAM   %s  <code>%s</code>\n", bar(it.MemPct, 100), Escape(it.MemHuman)))
		if it.MemPct > 0 {
			b.WriteString(fmt.Sprintf("           <i>%.1f%% do limite</i>\n", it.MemPct))
		}
	}
	return Truncate(b.String())
}

func Problems(items []ProblemItem) string {
	var b strings.Builder
	b.WriteString(Header("Problemas"))
	b.WriteString(Blank())

	if len(items) == 0 {
		b.WriteString(Divider())
		b.WriteString(Blank())
		b.WriteString("Nenhum problema no momento.\n")
		return Truncate(b.String())
	}

	b.WriteString(fmt.Sprintf("<b>%d</b> encontrado(s)\n", len(items)))
	b.WriteString(Blank())
	b.WriteString(Divider())

	for _, p := range items {
		b.WriteString(Blank())
		b.WriteString(fmt.Sprintf("<b>%s</b>\n", Escape(p.Name)))
		b.WriteString(fmt.Sprintf("     %s\n", Escape(p.Reason)))
		b.WriteString(fmt.Sprintf("     <i>%s</i>  ·  <code>%s</code>\n", Escape(p.Host), Escape(p.Severity)))
	}
	return Truncate(b.String())
}

func Daily(date time.Time, problems []ProblemItem, total, running int, top []MetricItem) string {
	var b strings.Builder
	b.WriteString(Header("Resumo diário"))
	b.WriteString(Blank())
	b.WriteString(Subtitle(date.Format("02/01/2006 · 15:04")))
	b.WriteString(Blank())
	b.WriteString(Divider())

	b.WriteString(Blank())
	b.WriteString("<b>Problemas</b>\n")
	b.WriteString(Blank())
	if len(problems) == 0 {
		b.WriteString("Nenhum problema.\n")
	} else {
		for _, p := range problems {
			b.WriteString(fmt.Sprintf("<b>%s</b>  ·  <code>%s</code>\n", Escape(p.Name), Escape(p.Severity)))
			b.WriteString(fmt.Sprintf("     %s\n", Escape(p.Reason)))
			b.WriteString(Blank())
		}
	}

	b.WriteString(Divider())
	b.WriteString(Blank())
	b.WriteString("<b>Containers</b>\n")
	b.WriteString(Blank())
	b.WriteString(fmt.Sprintf("Rodando  <b>%d</b> / %d\n", running, total))

	if len(top) > 0 {
		b.WriteString(Blank())
		b.WriteString("<b>Top uso</b>\n")
		for _, it := range top {
			b.WriteString(Blank())
			b.WriteString(fmt.Sprintf("<b>%s</b>\n", Escape(it.Name)))
			b.WriteString(fmt.Sprintf("     CPU <code>%.1f%%</code>  ·  RAM <code>%s</code>\n",
				it.CPUPct, Escape(it.MemHuman)))
		}
	}

	return Truncate(b.String())
}

func Help() string {
	var b strings.Builder
	b.WriteString(Header("Dockwatch"))
	b.WriteString(Blank())
	b.WriteString("Comandos disponíveis:\n")
	b.WriteString(Blank())
	b.WriteString(Divider())
	b.WriteString(Blank())
	b.WriteString("<b>/metric</b>\n")
	b.WriteString("CPU e RAM de todos os containers\n")
	b.WriteString(Blank())
	b.WriteString("<b>/problems</b>\n")
	b.WriteString("Problemas critical e warning agora\n")
	b.WriteString(Blank())
	b.WriteString("<b>/help</b>\n")
	b.WriteString("Esta mensagem\n")
	b.WriteString(Blank())
	b.WriteString(Divider())
	b.WriteString(Blank())
	b.WriteString("<i>Alertas automáticos e um resumo diário também chegam neste chat.</i>\n")
	return b.String()
}

func Collecting() string {
	return "Coletando métricas…\n"
}

func Error(msg string) string {
	return fmt.Sprintf("<b>Erro</b>\n\n%s\n", Escape(msg))
}

func UnknownCommand() string {
	return "Comando desconhecido.\n\nUse /help para ver as opções.\n"
}

func bar(pct, max float64) string {
	if max <= 0 {
		max = 100
	}
	n := int((pct / max) * 8)
	if n < 0 {
		n = 0
	}
	if n > 8 {
		n = 8
	}
	return strings.Repeat(barFull, n) + strings.Repeat(barEmpty, 8-n)
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
