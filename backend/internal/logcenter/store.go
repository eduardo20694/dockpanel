package logcenter

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	DefaultRetentionDays = 60
	MaxRetentionDays     = 60
	settingRetentionKey  = "retention_days"
)

type Entry struct {
	ID            int64  `json:"id"`
	HostID        string `json:"hostId"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	Stream        string `json:"stream"`
	TimestampMs   int64  `json:"timestampMs"`
	TimestampNano int64  `json:"-"` // dedupe only
	Message       string `json:"message"`
	Severity      string `json:"severity"`
}

type SearchParams struct {
	Query     string
	Container string
	Severity  string
	FromMs    int64
	ToMs      int64
	Limit     int
	Cursor    string // "ts_ms:id"
}

type SearchResult struct {
	Entries    []Entry `json:"entries"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

type Store struct {
	db   *sql.DB
	mu   sync.Mutex
	last map[string]int64 // hostID:containerID -> last ts_ms
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = os.Getenv("DOCKPANEL_DATA_DIR")
		if dir == "" {
			dir = "data"
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "logs.db")
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, last: make(map[string]int64)}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.loadLastSeen(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.ensureRetentionSetting(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS log_entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  host_id TEXT NOT NULL,
  container_id TEXT NOT NULL,
  container_name TEXT NOT NULL,
  stream TEXT NOT NULL,
  ts_ms INTEGER NOT NULL,
  message TEXT NOT NULL,
  severity TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_logs_container_ts ON log_entries(container_id, ts_ms);
CREATE INDEX IF NOT EXISTS idx_logs_severity_ts ON log_entries(severity, ts_ms);
CREATE INDEX IF NOT EXISTS idx_logs_host_ts ON log_entries(host_id, ts_ms);

CREATE VIRTUAL TABLE IF NOT EXISTS log_entries_fts USING fts5(
  message,
  content='log_entries',
  content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS log_entries_ai AFTER INSERT ON log_entries BEGIN
  INSERT INTO log_entries_fts(rowid, message) VALUES (new.id, new.message);
END;
CREATE TRIGGER IF NOT EXISTS log_entries_ad AFTER DELETE ON log_entries BEGIN
  INSERT INTO log_entries_fts(log_entries_fts, rowid, message) VALUES('delete', old.id, old.message);
END;

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`)
	return err
}

func (s *Store) ensureRetentionSetting() error {
	days := DefaultRetentionDays
	if v := strings.TrimSpace(os.Getenv("DOCKPANEL_LOG_RETENTION_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			days = clampRetention(n)
		}
	}
	var existing string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, settingRetentionKey).Scan(&existing)
	if err == sql.ErrNoRows {
		_, err = s.db.Exec(`INSERT INTO settings(key, value) VALUES(?, ?)`, settingRetentionKey, strconv.Itoa(days))
		return err
	}
	return err
}

func (s *Store) loadLastSeen() error {
	rows, err := s.db.Query(`SELECT host_id, container_id, MAX(ts_ms) FROM log_entries GROUP BY host_id, container_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var host, cid string
		var ts int64
		if err := rows.Scan(&host, &cid, &ts); err != nil {
			return err
		}
		s.last[host+":"+cid] = ts * int64(time.Millisecond)
	}
	return rows.Err()
}

func lastKey(hostID, containerID string) string {
	return hostID + ":" + containerID
}

// Insert persists a line if newer than last seen for that container. Returns false if skipped.
func (s *Store) Insert(e Entry) (bool, error) {
	if e.HostID == "" {
		e.HostID = "default"
	}
	if e.Stream == "" {
		e.Stream = "stdout"
	}
	if e.Severity == "" {
		e.Severity = "ok"
	}
	key := lastKey(e.HostID, e.ContainerID)
	nano := e.TimestampNano
	if nano == 0 {
		nano = e.TimestampMs * int64(time.Millisecond)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prev, ok := s.last[key]; ok && nano <= prev {
		return false, nil
	}
	res, err := s.db.Exec(
		`INSERT INTO log_entries(host_id, container_id, container_name, stream, ts_ms, message, severity)
		 VALUES(?,?,?,?,?,?,?)`,
		e.HostID, e.ContainerID, e.ContainerName, e.Stream, e.TimestampMs, e.Message, e.Severity,
	)
	if err != nil {
		return false, err
	}
	id, _ := res.LastInsertId()
	e.ID = id
	s.last[key] = nano
	return true, nil
}

func (s *Store) LastSeen(hostID, containerID string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	nano := s.last[lastKey(hostID, containerID)]
	if nano == 0 {
		return 0
	}
	return nano / int64(time.Millisecond)
}

func (s *Store) Search(p SearchParams) (*SearchResult, error) {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 100
	}
	var (
		args []interface{}
		where []string
	)
	where = append(where, "1=1")
	if p.Container != "" {
		where = append(where, "(e.container_id LIKE ? OR e.container_name LIKE ?)")
		like := "%" + p.Container + "%"
		args = append(args, like, like)
	}
	if p.Severity != "" && p.Severity != "all" {
		where = append(where, "e.severity = ?")
		args = append(args, p.Severity)
	}
	if p.FromMs > 0 {
		where = append(where, "e.ts_ms >= ?")
		args = append(args, p.FromMs)
	}
	if p.ToMs > 0 {
		where = append(where, "e.ts_ms <= ?")
		args = append(args, p.ToMs)
	}
	if curTs, curID, ok := parseCursor(p.Cursor); ok {
		where = append(where, "(e.ts_ms < ? OR (e.ts_ms = ? AND e.id < ?))")
		args = append(args, curTs, curTs, curID)
	}

	var q string
	if strings.TrimSpace(p.Query) != "" {
		fts := sanitizeFTS(p.Query)
		q = fmt.Sprintf(`
SELECT e.id, e.host_id, e.container_id, e.container_name, e.stream, e.ts_ms, e.message, e.severity
FROM log_entries e
JOIN log_entries_fts f ON f.rowid = e.id
WHERE %s AND log_entries_fts MATCH ?
ORDER BY e.ts_ms DESC, e.id DESC
LIMIT ?`, strings.Join(where, " AND "))
		args = append(args, fts, p.Limit+1)
	} else {
		q = fmt.Sprintf(`
SELECT e.id, e.host_id, e.container_id, e.container_name, e.stream, e.ts_ms, e.message, e.severity
FROM log_entries e
WHERE %s
ORDER BY e.ts_ms DESC, e.id DESC
LIMIT ?`, strings.Join(where, " AND "))
		args = append(args, p.Limit+1)
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.HostID, &e.ContainerID, &e.ContainerName, &e.Stream, &e.TimestampMs, &e.Message, &e.Severity); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	res := &SearchResult{Entries: out}
	if len(out) > p.Limit {
		last := out[p.Limit-1]
		res.Entries = out[:p.Limit]
		res.NextCursor = fmt.Sprintf("%d:%d", last.TimestampMs, last.ID)
	}
	if res.Entries == nil {
		res.Entries = []Entry{}
	}
	return res, nil
}

func (s *Store) EntriesInRange(fromMs, toMs int64, severities []string) ([]Entry, error) {
	q := `SELECT id, host_id, container_id, container_name, stream, ts_ms, message, severity
FROM log_entries WHERE ts_ms >= ? AND ts_ms <= ?`
	args := []interface{}{fromMs, toMs}
	if len(severities) > 0 {
		ph := make([]string, len(severities))
		for i, sev := range severities {
			ph[i] = "?"
			args = append(args, sev)
		}
		q += " AND severity IN (" + strings.Join(ph, ",") + ")"
	}
	q += " ORDER BY ts_ms ASC, id ASC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.HostID, &e.ContainerID, &e.ContainerName, &e.Stream, &e.TimestampMs, &e.Message, &e.Severity); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) PruneOlderThan(cutoff time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM log_entries WHERE ts_ms < ?`, cutoff.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) GetRetentionDays() int {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, settingRetentionKey).Scan(&v)
	if err != nil {
		return DefaultRetentionDays
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return DefaultRetentionDays
	}
	return clampRetention(n)
}

func (s *Store) SetRetentionDays(days int) error {
	days = clampRetention(days)
	_, err := s.db.Exec(
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		settingRetentionKey, strconv.Itoa(days),
	)
	return err
}

func clampRetention(n int) int {
	if n < 1 {
		return 1
	}
	if n > MaxRetentionDays {
		return MaxRetentionDays
	}
	return n
}

func parseCursor(c string) (ts, id int64, ok bool) {
	c = strings.TrimSpace(c)
	if c == "" {
		return 0, 0, false
	}
	parts := strings.SplitN(c, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	ts, err1 := strconv.ParseInt(parts[0], 10, 64)
	id, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return ts, id, true
}

func sanitizeFTS(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, `"`, "")
	parts := strings.Fields(q)
	if len(parts) == 0 {
		return `""`
	}
	for i, p := range parts {
		parts[i] = `"` + p + `"*`
	}
	return strings.Join(parts, " ")
}
