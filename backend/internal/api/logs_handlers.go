package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"dockpanel/internal/logcenter"
)

func (s *Server) searchLogs(w http.ResponseWriter, r *http.Request) {
	if s.LogStore == nil {
		writeErr(w, errLogCenterDisabled())
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	fromMs, _ := strconv.ParseInt(q.Get("from"), 10, 64)
	toMs, _ := strconv.ParseInt(q.Get("to"), 10, 64)
	// allow RFC3339 as well
	if fromMs == 0 && q.Get("from") != "" {
		if t, err := time.Parse(time.RFC3339, q.Get("from")); err == nil {
			fromMs = t.UnixMilli()
		}
	}
	if toMs == 0 && q.Get("to") != "" {
		if t, err := time.Parse(time.RFC3339, q.Get("to")); err == nil {
			toMs = t.UnixMilli()
		}
	}
	res, err := s.LogStore.Search(logcenter.SearchParams{
		Query:     q.Get("q"),
		Container: q.Get("container"),
		Severity:  q.Get("severity"),
		FromMs:    fromMs,
		ToMs:      toMs,
		Limit:     limit,
		Cursor:    q.Get("cursor"),
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, res)
}

func (s *Server) listLogIncidents(w http.ResponseWriter, r *http.Request) {
	if s.LogStore == nil {
		writeErr(w, errLogCenterDisabled())
		return
	}
	q := r.URL.Query()
	fromMs, _ := strconv.ParseInt(q.Get("from"), 10, 64)
	toMs, _ := strconv.ParseInt(q.Get("to"), 10, 64)
	windowMs, _ := strconv.ParseInt(q.Get("window"), 10, 64)
	if fromMs == 0 && q.Get("from") != "" {
		if t, err := time.Parse(time.RFC3339, q.Get("from")); err == nil {
			fromMs = t.UnixMilli()
		}
	}
	if toMs == 0 && q.Get("to") != "" {
		if t, err := time.Parse(time.RFC3339, q.Get("to")); err == nil {
			toMs = t.UnixMilli()
		}
	}
	if windowMs == 0 && q.Get("window") != "" {
		if d, err := time.ParseDuration(q.Get("window")); err == nil {
			windowMs = d.Milliseconds()
		}
	}
	inc, err := s.LogStore.Incidents(fromMs, toMs, windowMs)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"incidents": inc})
}

func (s *Server) getLogRetention(w http.ResponseWriter, r *http.Request) {
	if s.LogStore == nil {
		writeErr(w, errLogCenterDisabled())
		return
	}
	writeJSON(w, map[string]interface{}{
		"days":    s.LogStore.GetRetentionDays(),
		"maxDays": logcenter.MaxRetentionDays,
	})
}

func (s *Server) putLogRetention(w http.ResponseWriter, r *http.Request) {
	if s.LogStore == nil {
		writeErr(w, errLogCenterDisabled())
		return
	}
	var body struct {
		Days int `json:"days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	if err := s.LogStore.SetRetentionDays(body.Days); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{
		"days":    s.LogStore.GetRetentionDays(),
		"maxDays": logcenter.MaxRetentionDays,
	})
}

type simpleError string

func (e simpleError) Error() string { return string(e) }

func errLogCenterDisabled() error {
	return simpleError("log center indisponível")
}
