// Package envx reads Dockpilot configuration with a legacy fallback.
// DOCKERPILOT_<SUFFIX> wins when it is non-empty; otherwise DOCKPANEL_<SUFFIX> is used.
package envx

import (
	"os"
	"strings"
)

// Get returns a trimmed environment value.
func Get(suffix string) string {
	if v := strings.TrimSpace(os.Getenv("DOCKERPILOT_" + suffix)); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("DOCKPANEL_" + suffix))
}

// GetRaw preserves whitespace. Use it for passwords, JWT secrets and JSON blobs.
func GetRaw(suffix string) string {
	if v, ok := os.LookupEnv("DOCKERPILOT_" + suffix); ok && v != "" {
		return v
	}
	return os.Getenv("DOCKPANEL_" + suffix)
}

// DataDir is the on-disk state directory (metrics, ships, audit, orgs).
func DataDir() string {
	if d := Get("DATA_DIR"); d != "" {
		return d
	}
	return "data"
}

// ReadOnly reports whether mutations must be rejected.
func ReadOnly() bool {
	switch strings.ToLower(Get("READONLY")) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
