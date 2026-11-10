package scan

import "encoding/json"

// ToSARIF renders a Trivy-style report as SARIF 2.1.0.
func ToSARIF(rep Report) ([]byte, error) {
	var results []map[string]any
	rules := []map[string]any{}
	seen := map[string]bool{}
	for _, v := range rep.Vulnerabilities {
		level := "note"
		switch v.Severity {
		case "CRITICAL", "critical":
			level = "error"
		case "HIGH", "high":
			level = "error"
		case "MEDIUM", "medium":
			level = "warning"
		}
		results = append(results, map[string]any{
			"ruleId": v.ID,
			"level":  level,
			"message": map[string]string{"text": v.Title + " (" + v.PkgName + " " + v.Installed + ")"},
			"locations": []any{map[string]any{"physicalLocation": map[string]any{"artifactLocation": map[string]string{"uri": rep.Image}}}},
		})
		if !seen[v.ID] {
			seen[v.ID] = true
			rules = append(rules, map[string]any{"id": v.ID, "shortDescription": map[string]string{"text": v.Title}, "properties": map[string]string{"severity": v.Severity}})
		}
	}
	if results == nil {
		results = []map[string]any{}
	}
	doc := map[string]any{
		"version": "2.1.0",
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"runs": []any{map[string]any{
			"tool": map[string]any{"driver": map[string]any{"name": "dockpilot", "rules": rules}},
			"results": results,
		}},
	}
	return json.MarshalIndent(doc, "", "  ")
}

// BlockCriticalIncrease reports whether a newer image adds critical CVEs.
func BlockCriticalIncrease(current, next int, enabled bool) bool {
	return enabled && next > current
}
