package diagnostics

import (
	"regexp"
	"strings"
)

// errorPatterns são os sinais textuais mais comuns de problema em logs
// de aplicação, cobrindo várias linguagens/runtimes comuns.
var errorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(error|erro|fatal|panic|exception|traceback|segfault)\b`),
	regexp.MustCompile(`(?i)\bconnection refused\b`),
	regexp.MustCompile(`(?i)\baddress already in use\b`),
	regexp.MustCompile(`(?i)\bout of memory\b`),
	regexp.MustCompile(`(?i)\bpermission denied\b`),
	regexp.MustCompile(`(?i)\btimeout\b`),
	regexp.MustCompile(`(?i)\b(econnrefused|enotfound|eaddrinuse)\b`),
}

var criticalLogPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(fatal|panic|segfault|out of memory)\b`),
	regexp.MustCompile(`(?i)\boom\b`),
}

// ClassifyLogLine maps a log line to ok/warning/critical using shared patterns.
func ClassifyLogLine(msg string) Severity {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return SeverityOK
	}
	for _, p := range criticalLogPatterns {
		if p.MatchString(msg) {
			return SeverityCritical
		}
	}
	for _, p := range errorPatterns {
		if p.MatchString(msg) {
			return SeverityWarning
		}
	}
	return SeverityOK
}

// LogLineLooksLikeError reports whether the line matches any error pattern.
func LogLineLooksLikeError(msg string) bool {
	return ClassifyLogLine(msg) != SeverityOK
}
