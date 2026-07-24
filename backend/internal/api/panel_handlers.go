package api

import (
	"net/http"
	"strconv"
	"time"
)

func (s *Server) metricsSeries(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, []interface{}{})
		return
	}
	hostID := r.URL.Query().Get("host")
	if hostID == "" {
		hostID = s.Hosts.DefaultID()
	}
	containerID := r.URL.Query().Get("container")
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 {
		hours = 24
	}
	pts, err := s.Store.MetricsHistory(hostID, containerID, time.Now().Add(-time.Duration(hours)*time.Hour), 2000)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]map[string]interface{}, 0, len(pts))
	for _, p := range pts {
		out = append(out, map[string]interface{}{
			"t":   time.UnixMilli(p.Timestamp).UTC().Format(time.RFC3339),
			"cpu": p.CPUPct,
			"mem": p.MemPct,
		})
	}
	writeJSON(w, out)
}

func (s *Server) metricsHost(w http.ResponseWriter, r *http.Request) {
	s.metricsSeries(w, r)
}
