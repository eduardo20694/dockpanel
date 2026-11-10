package scan

import (
	"strings"
	"testing"
)

func TestSARIFAndPolicy(t *testing.T) {
	raw, err := ToSARIF(Report{Image: "api:1", Vulnerabilities: []Vulnerability{{ID: "CVE-2024-1", PkgName: "openssl", Installed: "1", Severity: "CRITICAL", Title: "overflow"}}})
	if err != nil || !strings.Contains(string(raw), `"version": "2.1.0"`) || !strings.Contains(string(raw), "CVE-2024-1") {
		t.Fatal(string(raw), err)
	}
	if !BlockCriticalIncrease(1, 3, true) {
		t.Fatal("should block")
	}
	if BlockCriticalIncrease(3, 1, true) || BlockCriticalIncrease(1, 3, false) {
		t.Fatal("should allow")
	}
}
