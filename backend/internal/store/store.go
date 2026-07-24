package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Store struct {
	dir string
	mu  sync.Mutex
}

func Open(path string) (*Store, error) {
	dir := path
	if dir == "" {
		dir = os.Getenv("DOCKPANEL_DATA_DIR")
		if dir == "" {
			dir = "data"
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) Close() error { return nil }

type metricRow struct {
	HostID       string  `json:"hostId"`
	ContainerID  string  `json:"containerId"`
	ContainerName string `json:"containerName"`
	CPUPct       float64 `json:"cpuPct"`
	MemPct       float64 `json:"memPct"`
	MemUsage     uint64  `json:"memUsage"`
	RestartCount int     `json:"restartCount"`
	Timestamp    int64   `json:"timestamp"`
}

type MetricSample struct {
	CPUPct       float64 `json:"cpuPct"`
	MemPct       float64 `json:"memPct"`
	MemUsage     uint64  `json:"memUsage"`
	RestartCount int     `json:"restartCount"`
	Timestamp    int64   `json:"timestamp"`
}

func (s *Store) RecordMetric(hostID, containerID, name string, cpu, memPct float64, memUsage uint64, restartCount int) error {
	return s.append("metrics.jsonl", metricRow{
		HostID: hostID, ContainerID: containerID, ContainerName: name,
		CPUPct: cpu, MemPct: memPct, MemUsage: memUsage, RestartCount: restartCount,
		Timestamp: time.Now().UnixMilli(),
	})
}

type restartRow struct {
	HostID        string `json:"hostId"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	RestartCount  int    `json:"restartCount"`
	Timestamp     int64  `json:"timestamp"`
}

func (s *Store) RecordRestart(hostID, containerID, name string, count int) error {
	return s.append("restarts.jsonl", restartRow{
		HostID: hostID, ContainerID: containerID, ContainerName: name,
		RestartCount: count, Timestamp: time.Now().UnixMilli(),
	})
}

type alertRow struct {
	HostID        string `json:"hostId"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	Severity      string `json:"severity"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	Timestamp     int64  `json:"timestamp"`
}

type AlertRecord struct {
	HostID        string `json:"hostId"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	Severity      string `json:"severity"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	Timestamp     int64  `json:"timestamp"`
}

func (s *Store) RecordAlert(hostID, containerID, name, severity, title, body string) error {
	return s.append("alerts.jsonl", alertRow{
		HostID: hostID, ContainerID: containerID, ContainerName: name,
		Severity: severity, Title: title, Body: body, Timestamp: time.Now().UnixMilli(),
	})
}

func (s *Store) MetricsHistory(hostID, containerID string, since time.Time, limit int) ([]MetricSample, error) {
	rows, err := s.readMetrics()
	if err != nil {
		return nil, err
	}
	cutoff := since.UnixMilli()
	var out []MetricSample
	for _, r := range rows {
		if r.HostID != hostID || r.Timestamp < cutoff {
			continue
		}
		if containerID != "" && r.ContainerID != containerID {
			continue
		}
		out = append(out, MetricSample{
			CPUPct: r.CPUPct, MemPct: r.MemPct, MemUsage: r.MemUsage,
			RestartCount: r.RestartCount, Timestamp: r.Timestamp,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

type RestartEvent struct {
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	RestartCount  int    `json:"restartCount"`
	Timestamp     int64  `json:"timestamp"`
}

func (s *Store) RestartHistory(hostID, containerID string, since time.Time, limit int) ([]RestartEvent, error) {
	rows, err := s.readRestarts()
	if err != nil {
		return nil, err
	}
	cutoff := since.UnixMilli()
	var out []RestartEvent
	for _, r := range rows {
		if r.HostID != hostID || r.Timestamp < cutoff {
			continue
		}
		if containerID != "" && r.ContainerID != containerID {
			continue
		}
		out = append(out, RestartEvent{
			ContainerID: r.ContainerID, ContainerName: r.ContainerName,
			RestartCount: r.RestartCount, Timestamp: r.Timestamp,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) RecentAlerts(limit int) ([]AlertRecord, error) {
	rows, err := s.readAlerts()
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Timestamp > rows[j].Timestamp })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]AlertRecord, len(rows))
	for i, r := range rows {
		out[i] = AlertRecord(r)
	}
	return out, nil
}

func (s *Store) PruneOlderThan(d time.Duration) error {
	cutoff := time.Now().Add(-d).UnixMilli()
	if err := s.pruneFile("metrics.jsonl", cutoff, func(line []byte) bool {
		var r metricRow
		if json.Unmarshal(line, &r) != nil {
			return true
		}
		return r.Timestamp >= cutoff
	}); err != nil {
		return err
	}
	return s.pruneFile("restarts.jsonl", cutoff, func(line []byte) bool {
		var r restartRow
		if json.Unmarshal(line, &r) != nil {
			return true
		}
		return r.Timestamp >= cutoff
	})
}

func (s *Store) append(name string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(s.dir, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func (s *Store) readMetrics() ([]metricRow, error) {
	return readJSONL[metricRow](s.dir, "metrics.jsonl")
}

func (s *Store) readRestarts() ([]restartRow, error) {
	return readJSONL[restartRow](s.dir, "restarts.jsonl")
}

func (s *Store) readAlerts() ([]alertRow, error) {
	return readJSONL[alertRow](s.dir, "alerts.jsonl")
}

func readJSONL[T any](dir, name string) ([]T, error) {
	path := filepath.Join(dir, name)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var row T
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			continue
		}
		out = append(out, row)
	}
	return out, sc.Err()
}

func (s *Store) pruneFile(name string, cutoff int64, keep func([]byte) bool) error {
	path := filepath.Join(s.dir, name)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var kept [][]byte
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		if keep(line) {
			kept = append(kept, line)
		}
	}
	var buf []byte
	for _, line := range kept {
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	return os.WriteFile(path, buf, 0644)
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func DefaultRetention() time.Duration {
	if v := os.Getenv("DOCKPANEL_HISTORY_DAYS"); v != "" {
		var days int
		if _, err := fmt.Sscanf(v, "%d", &days); err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}
	return 7 * 24 * time.Hour
}
