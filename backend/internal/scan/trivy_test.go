package scan

import (
	"strings"
	"testing"
)

func TestNormalizeImageRef(t *testing.T) {
	got := normalizeImageRef("redecoop%2Fredecoop%3A1.0.3")
	if got != "redecoop/redecoop:1.0.3" {
		t.Fatalf("got %q", got)
	}
	if normalizeImageRef("nginx:alpine") != "nginx:alpine" {
		t.Fatal("plain ref should stay unchanged")
	}
}

func TestParseTrivyJSON(t *testing.T) {
	raw := `{"Results":[{"Vulnerabilities":[
		{"VulnerabilityID":"CVE-2024-1","PkgName":"openssl","InstalledVersion":"1.1","FixedVersion":"1.2","Severity":"CRITICAL","Title":"RCE"},
		{"VulnerabilityID":"CVE-2024-2","PkgName":"curl","InstalledVersion":"7.0","FixedVersion":"7.1","Severity":"HIGH","Title":"Leak"}
	]}]}`

	report, err := parseTrivyJSON("nginx:alpine", raw)
	if err != nil {
		t.Fatal(err)
	}
	if report.VulnCount != 2 || report.CriticalCount != 1 || report.HighCount != 1 {
		t.Fatalf("counts: %+v", report)
	}
	if report.Vulnerabilities[0].ID != "CVE-2024-1" {
		t.Fatalf("vuln=%+v", report.Vulnerabilities[0])
	}
}

func TestParseTrivyJSONTruncates(t *testing.T) {
	raw := `{"Results":[{"Vulnerabilities":[`
	for i := 0; i < 60; i++ {
		if i > 0 {
			raw += ","
		}
		raw += `{"VulnerabilityID":"CVE-` + string(rune('A'+i%26)) + `","PkgName":"p","InstalledVersion":"1","FixedVersion":"2","Severity":"LOW","Title":"t"}`
	}
	raw += `]}]}`

	report, err := parseTrivyJSON("big", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Vulnerabilities) != 50 || report.RawNote == "" {
		t.Fatalf("expected truncation, got %d note=%q", len(report.Vulnerabilities), report.RawNote)
	}
}

func TestParseTrivyJSONInvalid(t *testing.T) {
	_, err := parseTrivyJSON("x", "{")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseTrivyJSONEmpty(t *testing.T) {
	report, err := parseTrivyJSON("nginx", `{"Results":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	if report.VulnCount != 0 {
		t.Fatalf("expected 0 vulns")
	}
}

func TestParseTrivyJSONStripsNoise(t *testing.T) {
	raw := "pulling...\n{\"Results\":[]}\n"
	report, err := parseTrivyJSON("x", raw)
	if err != nil {
		t.Fatal(err)
	}
	if report.VulnCount != 0 {
		t.Fatalf("expected 0")
	}
}

func TestDockerSockMount(t *testing.T) {
	m := dockerSockMount()
	if !strings.Contains(m, "docker.sock") {
		t.Fatalf("mount %q", m)
	}
}
