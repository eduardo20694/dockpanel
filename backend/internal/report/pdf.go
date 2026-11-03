package report

import (
	"fmt"
	"strings"
	"time"
)

func pdf(in Input) []byte {
	var lines []string
	lines = append(lines, "DOCKPILOT", "Relatorio de atendimento", "")
	lines = append(lines, "Titulo: "+in.Title)
	lines = append(lines, "Quando: "+in.When.UTC().Format(time.RFC3339))
	if in.Session != "" {
		lines = append(lines, "Sessao: "+in.Session)
	}
	lines = append(lines, "", "Incidente", blank(in.Incident), "", "Problemas encontrados")
	if len(in.Problems) == 0 {
		lines = append(lines, "- nenhum registro")
	}
	for _, p := range in.Problems {
		lines = append(lines, "- "+p)
	}
	lines = append(lines, "", "Acoes feitas")
	if len(in.Actions) == 0 {
		lines = append(lines, "- nenhum registro")
	}
	for _, a := range in.Actions {
		lines = append(lines, "- "+a)
	}
	lines = append(lines, "", "Estado final", blank(in.FinalState))

	var stream strings.Builder
	stream.WriteString("0.05 0.05 0.04 rg\n0 760 595 82 re f\n")
	stream.WriteString("1 1 1 rg\nBT /F1 18 Tf 48 800 Td (DOCKPILOT) Tj ET\n")
	stream.WriteString("BT /F1 11 Tf 48 780 Td (Relatorio de atendimento) Tj ET\n")
	stream.WriteString("0.09 0.09 0.11 rg\n")
	y := 730
	for _, line := range lines {
		if y < 48 {
			break
		}
		if line == "DOCKPILOT" || line == "Relatorio de atendimento" {
			continue
		}
		fmt.Fprintf(&stream, "BT /F1 11 Tf 48 %d Td (%s) Tj ET\n", y, pdfText(line))
		y -= 16
	}
	content := stream.String()
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs)+1)
	for i, obj := range objs {
		offs[i+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(objs)+1)
	b.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objs); i++ {
		fmt.Fprintf(&b, "%010d 00000 n \n", offs[i])
	}
	fmt.Fprintf(&b, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return []byte(b.String())
}

func pdfText(s string) string {
	var b strings.Builder
	for _, r := range latin(s) {
		switch r {
		case '\\', '(', ')':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			if r < 32 || r > 255 {
				b.WriteByte('?')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func latin(s string) string {
	r := strings.NewReplacer(
		"á", "\xe1", "à", "\xe0", "â", "\xe2", "ã", "\xe3",
		"é", "\xe9", "ê", "\xea", "í", "\xed",
		"ó", "\xf3", "ô", "\xf4", "õ", "\xf5", "ú", "\xfa",
		"ç", "\xe7", "Á", "\xc1", "É", "\xc9", "Í", "\xcd",
		"Ó", "\xd3", "Ú", "\xda", "Ã", "\xc3", "Õ", "\xd5", "Ç", "\xc7",
	)
	return r.Replace(s)
}
