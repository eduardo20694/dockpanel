package secrets

import (
	"regexp"
	"strconv"
	"strings"
)

// Finding is a plaintext secret left in a compose file.
type Finding struct {
	Line       int    `json:"line"`
	Key        string `json:"key"`
	Suggestion string `json:"suggestion"`
}

var secretLine = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|private[_-]?key)(['"\s]*[:=]['"\s]*)([^#\s"']+)`)

// ScanCompose flags literal secrets and ignores ${VAR} references.
func ScanCompose(text string) []Finding {
	var out []Finding
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		m := secretLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		val := strings.Trim(m[3], `"'`)
		if val == "" || strings.HasPrefix(val, "${") || strings.HasPrefix(val, "{{") {
			continue
		}
		key := m[1]
		out = append(out, Finding{
			Line:       i + 1,
			Key:        key,
			Suggestion: "A linha " + strconv.Itoa(i+1) + " tem " + key + " em texto. Grave no cofre e troque por ${" + strings.ToUpper(key) + "}.",
		})
	}
	if out == nil {
		out = []Finding{}
	}
	return out
}

var redactRE = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|private[_-]?key)(\s*[:=]\s*)([^\s]+)`)

// Redact masks secret-looking values in diffs and audit text.
func Redact(s string) string {
	return redactRE.ReplaceAllString(s, "$1$2***")
}
