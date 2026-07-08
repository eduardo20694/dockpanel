package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Vulnerability struct {
	ID          string `json:"id"`
	PkgName     string `json:"pkgName"`
	Installed   string `json:"installedVersion"`
	Fixed       string `json:"fixedVersion"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
}

type Report struct {
	Image          string          `json:"image"`
	Scanner        string          `json:"scanner"`
	VulnCount      int             `json:"vulnCount"`
	CriticalCount  int             `json:"criticalCount"`
	HighCount      int             `json:"highCount"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	RawNote        string          `json:"rawNote,omitempty"`
}

type trivyJSON struct {
	Results []struct {
		Vulnerabilities []struct {
			VulnerabilityID  string `json:"VulnerabilityID"`
			PkgName          string `json:"PkgName"`
			InstalledVersion string `json:"InstalledVersion"`
			FixedVersion     string `json:"FixedVersion"`
			Severity         string `json:"Severity"`
			Title            string `json:"Title"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

func TrivyAvailable() bool {
	_, err := exec.LookPath("trivy")
	return err == nil
}

func ScanImage(ctx context.Context, imageRef string) (*Report, error) {
	if !TrivyAvailable() {
		return &Report{
			Image:   imageRef,
			Scanner: "trivy",
			RawNote: "trivy não encontrado no PATH — instale: https://aquasecurity.github.io/trivy/",
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "trivy", "image", "--scanners", "vuln", "--format", "json", "--quiet", imageRef)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("trivy falhou: %s", strings.TrimSpace(out.String()))
	}

	var raw trivyJSON
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		return nil, err
	}

	report := &Report{Image: imageRef, Scanner: "trivy"}
	for _, r := range raw.Results {
		for _, v := range r.Vulnerabilities {
			sev := strings.ToUpper(v.Severity)
			report.Vulnerabilities = append(report.Vulnerabilities, Vulnerability{
				ID:        v.VulnerabilityID,
				PkgName:   v.PkgName,
				Installed: v.InstalledVersion,
				Fixed:     v.FixedVersion,
				Severity:  sev,
				Title:     v.Title,
			})
			report.VulnCount++
			switch sev {
			case "CRITICAL":
				report.CriticalCount++
			case "HIGH":
				report.HighCount++
			}
		}
	}
	if len(report.Vulnerabilities) > 50 {
		report.Vulnerabilities = report.Vulnerabilities[:50]
		report.RawNote = "mostrando top 50 CVEs"
	}
	return report, nil
}
