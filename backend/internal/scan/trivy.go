package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Vulnerability struct {
	ID        string `json:"id"`
	PkgName   string `json:"pkgName"`
	Installed string `json:"installedVersion"`
	Fixed     string `json:"fixedVersion"`
	Severity  string `json:"severity"`
	Title     string `json:"title"`
}

type Report struct {
	Image           string          `json:"image"`
	Scanner         string          `json:"scanner"`
	VulnCount       int             `json:"vulnCount"`
	CriticalCount   int             `json:"criticalCount"`
	HighCount       int             `json:"highCount"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	RawNote         string          `json:"rawNote,omitempty"`
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

const trivyDockerImage = "aquasec/trivy:latest"

func trivyBinary() string {
	if p := strings.TrimSpace(os.Getenv("TRIVY_PATH")); p != "" {
		return p
	}
	if p, err := exec.LookPath("trivy"); err == nil {
		return p
	}
	return ""
}

func dockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// TrivyAvailable indica se dá para escanear (binário local ou via docker).
func TrivyAvailable() bool {
	return trivyBinary() != "" || dockerAvailable()
}

func normalizeImageRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ref
	}
	if decoded, err := url.PathUnescape(ref); err == nil {
		ref = decoded
	}
	if decoded, err := url.QueryUnescape(ref); err == nil {
		ref = decoded
	}
	return ref
}

func ScanImage(ctx context.Context, imageRef string) (*Report, error) {
	imageRef = normalizeImageRef(imageRef)
	if imageRef == "" {
		return nil, fmt.Errorf("informe a referência da imagem")
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if bin := trivyBinary(); bin != "" {
		report, err := runTrivyLocal(ctx, bin, imageRef)
		if err == nil {
			return report, nil
		}
		// Se o binário falhar e houver docker, tenta fallback.
		if !dockerAvailable() {
			return nil, err
		}
	}

	if dockerAvailable() {
		report, err := runTrivyDocker(ctx, imageRef)
		if err != nil {
			return nil, err
		}
		if report.RawNote == "" {
			report.RawNote = "scan via docker (" + trivyDockerImage + ")"
		}
		return report, nil
	}

	return &Report{
		Image:   imageRef,
		Scanner: "trivy",
		RawNote: "trivy não encontrado — instale o binário (https://aquasecurity.github.io/trivy/) ou tenha o Docker no PATH para fallback automático",
	}, nil
}

func runTrivyLocal(ctx context.Context, bin, imageRef string) (*Report, error) {
	cmd := exec.CommandContext(ctx, bin, "image", "--scanners", "vuln", "--format", "json", "--quiet", imageRef)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("trivy falhou: %s", strings.TrimSpace(out.String()))
	}
	return parseTrivyJSON(imageRef, out.String())
}

func runTrivyDocker(ctx context.Context, imageRef string) (*Report, error) {
	args := []string{
		"run", "--rm",
		"-v", dockerSockMount(),
		trivyDockerImage,
		"image", "--scanners", "vuln", "--format", "json", "--quiet",
		imageRef,
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("trivy (docker) falhou: %s", msg)
	}
	return parseTrivyJSON(imageRef, out.String())
}

// dockerSockMount devolve -v para o socket do Docker (Linux/macOS/Windows Desktop).
func dockerSockMount() string {
	if runtime.GOOS == "windows" {
		// Docker Desktop no Windows aceita este path no lado do container host.
		return "//var/run/docker.sock:/var/run/docker.sock"
	}
	sock := "/var/run/docker.sock"
	if p := strings.TrimSpace(os.Getenv("DOCKER_SOCK")); p != "" {
		sock = p
	}
	return sock + ":/var/run/docker.sock"
}

func parseTrivyJSON(imageRef, output string) (*Report, error) {
	output = strings.TrimSpace(output)
	// docker/trivy podem misturar progresso no stderr+stdout; extrai o objeto JSON.
	if i := strings.Index(output, "{"); i >= 0 {
		if j := strings.LastIndex(output, "}"); j > i {
			output = output[i : j+1]
		}
	}
	var raw trivyJSON
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return nil, fmt.Errorf("parse trivy json: %w", err)
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
