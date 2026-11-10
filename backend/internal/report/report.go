// Package report builds a Dockpilot attendance report as Markdown and PDF.
package report

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Input struct {
	Title      string
	Session    string
	Incident   string
	Problems   []string
	Actions    []string
	FinalState string
	When       time.Time
}

func Build(in Input) (string, []byte) {
	if in.When.IsZero() {
		in.When = time.Now().UTC()
	}
	if in.Title == "" {
		in.Title = "Atendimento"
	}
	md := markdown(in)
	return md, pdf(in)
}

func Save(dir string, in Input) (string, string, []byte, error) {
	if dir == "" {
		dir = "data"
	}
	dir = filepath.Join(dir, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", nil, err
	}
	id := newID()
	md, bin := Build(in)
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(md), 0o644); err != nil {
		return "", "", nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, id+".pdf"), bin, 0o644); err != nil {
		return "", "", nil, err
	}
	return id, md, bin, nil
}

func Load(dir, id, ext string) ([]byte, error) {
	if ext != "md" && ext != "pdf" {
		ext = "md"
	}
	return os.ReadFile(filepath.Join(dir, "reports", id+"."+ext))
}

func markdown(in Input) string {
	var b strings.Builder
	b.WriteString("# Relatório de atendimento\n\n")
	b.WriteString("**Dockpilot**\n\n")
	b.WriteString("- Título: " + in.Title + "\n")
	b.WriteString("- Quando: " + in.When.UTC().Format(time.RFC3339) + "\n")
	if in.Session != "" {
		b.WriteString("- Sessão: " + in.Session + "\n")
	}
	b.WriteString("\n## Incidente\n\n")
	b.WriteString(blank(in.Incident) + "\n\n")
	b.WriteString("## Problemas encontrados\n\n")
	writeList(&b, in.Problems)
	b.WriteString("\n## Ações feitas\n\n")
	writeList(&b, in.Actions)
	b.WriteString("\n## Estado final\n\n")
	b.WriteString(blank(in.FinalState) + "\n")
	return b.String()
}

func writeList(b *strings.Builder, items []string) {
	if len(items) == 0 {
		b.WriteString("- nenhum registro\n")
		return
	}
	for _, it := range items {
		b.WriteString("- " + it + "\n")
	}
}

func blank(s string) string {
	if strings.TrimSpace(s) == "" {
		return "não informado"
	}
	return s
}

func newID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
