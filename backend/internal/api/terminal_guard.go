package api

import (
	"errors"
	"strings"
	"unicode/utf8"

	"dockerpilot/internal/envx"
	"dockerpilot/internal/secrets"
)

func terminalPolicy(role string) error {
	if envx.ReadOnly() {
		return errors.New("modo somente leitura")
	}
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "viewer", "auditor":
		return errors.New("este papel não abre terminal")
	default:
		return nil
	}
}

// consumeTerminalInput appends bytes and returns completed command lines.
func consumeTerminalInput(buf *strings.Builder, msg []byte) []string {
	var done []string
	for len(msg) > 0 {
		r, size := utf8.DecodeRune(msg)
		msg = msg[size:]
		switch r {
		case '\r', '\n':
			cmd := strings.TrimSpace(buf.String())
			buf.Reset()
			if cmd != "" {
				done = append(done, secrets.Redact(cmd))
			}
		case 127, '\b':
			s := buf.String()
			if s != "" {
				_, w := utf8.DecodeLastRuneInString(s)
				buf.Reset()
				buf.WriteString(s[:len(s)-w])
			}
		default:
			if r >= 32 {
				buf.WriteRune(r)
			}
		}
	}
	return done
}
