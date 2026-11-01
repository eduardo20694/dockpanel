package main

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"dockerpilot/internal/apikey"
	"dockerpilot/internal/audit"
	"dockerpilot/internal/envx"

	"github.com/mark3labs/mcp-go/server"
)

type limiter struct {
	mu sync.Mutex
	n  map[string]int
	t  map[string]time.Time
}

func (l *limiter) allow(id string, max int) bool {
	if max <= 0 {
		max = 120
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.n == nil {
		l.n = map[string]int{}
		l.t = map[string]time.Time{}
	}
	if time.Since(l.t[id]) > time.Minute {
		l.n[id] = 0
		l.t[id] = time.Now()
	}
	l.n[id]++
	return l.n[id] <= max
}

func serveHTTP(addr string, mcpServer *server.MCPServer, dataDir string, aud *audit.Log) error {
	keys, err := apikey.Open(dataDir)
	if err != nil {
		return err
	}
	return http.ListenAndServe(addr, mcpHandler(mcpServer, keys, aud))
}

func mcpHandler(mcpServer *server.MCPServer, keys *apikey.Store, aud *audit.Log) http.Handler {
	stream := server.NewStreamableHTTPServer(mcpServer, server.WithEndpointPath("/mcp"))
	sse := server.NewSSEServer(mcpServer, server.WithSSEEndpoint("/sse"), server.WithMessageEndpoint("/message"))
	lim := &limiter{}
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, err := keys.Authenticate(bearer(r))
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"` + err.Error() + `"}`))
				return
			}
			max := 120
			if v := envx.Get("MCP_RATE_PER_MIN"); v != "" {
				if n := atoi(v); n > 0 {
					max = n
				}
			}
			if !lim.allow(p.KeyID, max) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"limite de requisições da chave"}`))
				return
			}
			keys.Touch(p.KeyID)
			if aud != nil {
				_ = aud.Append(audit.Event{At: time.Now().UTC(), Actor: p.UserID, OrgID: p.OrgID, Action: "MCP " + r.Method + " " + r.URL.Path, Status: 200, Detail: "key:" + p.KeyID})
			}
			next.ServeHTTP(w, r.WithContext(apikey.WithPrincipal(r.Context(), p)))
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", auth(stream))
	mux.Handle("/sse", auth(sse))
	mux.Handle("/message", auth(sse))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	return mux
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
