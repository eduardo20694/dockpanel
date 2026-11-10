// Package apikey stores per-user MCP credentials. Only a SHA-256 hash is kept.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrUnauthorized = errors.New("chave inválida")
	ErrRevoked      = errors.New("chave revogada")
	ErrExpired      = errors.New("chave expirada")
	ErrForbidden    = errors.New("esta chave não tem permissão para esta ferramenta")
)

type Key struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"orgId"`
	UserID     string     `json:"userId"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Role       string     `json:"role"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	Stacks     []string   `json:"stacks,omitempty"`
	Hosts      []string   `json:"hosts,omitempty"`
	Guest      bool       `json:"guest,omitempty"`
	hash       string
}

type Principal struct {
	KeyID  string
	OrgID  string
	UserID string
	Role   string
	Name   string
	Stacks []string
	Hosts  []string
	Guest  bool
}

type Store struct {
	mu sync.Mutex
	db *sql.DB
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "apikeys.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
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
CREATE TABLE IF NOT EXISTS api_keys (
  id TEXT PRIMARY KEY,
  org_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  prefix TEXT NOT NULL,
  hash TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL,
  expires_at INTEGER,
  revoked_at INTEGER,
  last_used_at INTEGER,
  created_at INTEGER NOT NULL,
  stacks TEXT NOT NULL DEFAULT '',
  hosts TEXT NOT NULL DEFAULT '',
  guest INTEGER NOT NULL DEFAULT 0
);`)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE api_keys ADD COLUMN stacks TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE api_keys ADD COLUMN hosts TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE api_keys ADD COLUMN guest INTEGER NOT NULL DEFAULT 0`,
	} {
		_, _ = s.db.Exec(stmt)
	}
	return nil
}

// Issue returns the plaintext token once. It is never stored.
func (s *Store) Issue(orgID, userID, name, role string, ttl time.Duration) (string, Key, error) {
	if orgID == "" {
		orgID = "default"
	}
	role = strings.ToLower(strings.TrimSpace(role))
	if !validRole(role) {
		return "", Key{}, errors.New("papel inválido")
	}
	if strings.TrimSpace(name) == "" {
		return "", Key{}, errors.New("nome obrigatório")
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", Key{}, err
	}
	token := "dp_live_" + hex.EncodeToString(raw)
	k := Key{
		ID: newID(), OrgID: orgID, UserID: userID, Name: strings.TrimSpace(name),
		Prefix: token[:16], Role: role, CreatedAt: time.Now().UTC(), hash: hashToken(token),
	}
	if ttl > 0 {
		exp := time.Now().UTC().Add(ttl)
		k.ExpiresAt = &exp
	}
	var exp, rev any
	if k.ExpiresAt != nil {
		exp = k.ExpiresAt.UnixMilli()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO api_keys(id,org_id,user_id,name,prefix,hash,role,expires_at,revoked_at,last_used_at,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		k.ID, k.OrgID, k.UserID, k.Name, k.Prefix, k.hash, k.Role, exp, rev, nil, k.CreatedAt.UnixMilli())
	if err != nil {
		return "", Key{}, err
	}
	return token, k, nil
}

func (s *Store) Authenticate(token string) (Principal, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Principal{}, ErrUnauthorized
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var k Key
	var exp, rev, used sql.NullInt64
	var created int64
	var stacks, hosts string
	var guest int
	err := s.db.QueryRow(`SELECT id,org_id,user_id,name,prefix,role,expires_at,revoked_at,last_used_at,created_at,stacks,hosts,guest FROM api_keys WHERE hash=?`, hashToken(token)).
		Scan(&k.ID, &k.OrgID, &k.UserID, &k.Name, &k.Prefix, &k.Role, &exp, &rev, &used, &created, &stacks, &hosts, &guest)
	if err == sql.ErrNoRows {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, err
	}
	k.CreatedAt = time.UnixMilli(created).UTC()
	if exp.Valid {
		t := time.UnixMilli(exp.Int64).UTC()
		k.ExpiresAt = &t
	}
	if rev.Valid {
		t := time.UnixMilli(rev.Int64).UTC()
		k.RevokedAt = &t
	}
	if k.RevokedAt != nil {
		return Principal{}, ErrRevoked
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return Principal{}, ErrExpired
	}
	return Principal{
		KeyID: k.ID, OrgID: k.OrgID, UserID: k.UserID, Role: k.Role, Name: k.Name,
		Stacks: splitList(stacks), Hosts: splitList(hosts), Guest: guest == 1,
	}, nil
}

// IssueGuest creates a short-lived consultant key. Hours must be 1–72 and at least one stack or host is required.
func (s *Store) IssueGuest(orgID, name, role string, hours int, stacks, hosts []string) (string, Key, error) {
	if hours < 1 || hours > 72 {
		return "", Key{}, errors.New("validade entre 1 e 72 horas")
	}
	stacks, hosts = cleanList(stacks), cleanList(hosts)
	if len(stacks) == 0 && len(hosts) == 0 {
		return "", Key{}, errors.New("informe ao menos uma stack ou um host")
	}
	if orgID == "" {
		orgID = "default"
	}
	role = strings.ToLower(strings.TrimSpace(role))
	if !validRole(role) {
		return "", Key{}, errors.New("papel inválido")
	}
	if strings.TrimSpace(name) == "" {
		return "", Key{}, errors.New("nome obrigatório")
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", Key{}, err
	}
	token := "dp_live_" + hex.EncodeToString(raw)
	exp := time.Now().UTC().Add(time.Duration(hours) * time.Hour)
	k := Key{
		ID: newID(), OrgID: orgID, UserID: "guest:" + strings.TrimSpace(name), Name: strings.TrimSpace(name),
		Prefix: token[:16], Role: role, CreatedAt: time.Now().UTC(), ExpiresAt: &exp,
		Stacks: stacks, Hosts: hosts, Guest: true, hash: hashToken(token),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO api_keys(id,org_id,user_id,name,prefix,hash,role,expires_at,revoked_at,last_used_at,created_at,stacks,hosts,guest)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,1)`,
		k.ID, k.OrgID, k.UserID, k.Name, k.Prefix, k.hash, k.Role, exp.UnixMilli(), nil, nil, k.CreatedAt.UnixMilli(),
		strings.Join(stacks, ","), strings.Join(hosts, ","))
	if err != nil {
		return "", Key{}, err
	}
	return token, k, nil
}

func (s *Store) Touch(id string) {
	if s == nil || id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(`UPDATE api_keys SET last_used_at=? WHERE id=?`, time.Now().UnixMilli(), id)
}

func (s *Store) Revoke(orgID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE api_keys SET revoked_at=? WHERE id=? AND org_id=? AND revoked_at IS NULL`, time.Now().UnixMilli(), id, orgID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("chave não encontrada")
	}
	return nil
}

func (s *Store) List(orgID string) ([]Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id,org_id,user_id,name,prefix,role,expires_at,revoked_at,last_used_at,created_at,stacks,hosts,guest FROM api_keys WHERE org_id=? ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Key
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	if out == nil {
		out = []Key{}
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanKey(rows scanner) (Key, error) {
	var k Key
	var exp, rev, used sql.NullInt64
	var created int64
	var stacks, hosts string
	var guest int
	err := rows.Scan(&k.ID, &k.OrgID, &k.UserID, &k.Name, &k.Prefix, &k.Role, &exp, &rev, &used, &created, &stacks, &hosts, &guest)
	if err != nil {
		return Key{}, err
	}
	k.CreatedAt = time.UnixMilli(created).UTC()
	if exp.Valid {
		t := time.UnixMilli(exp.Int64).UTC()
		k.ExpiresAt = &t
	}
	if rev.Valid {
		t := time.UnixMilli(rev.Int64).UTC()
		k.RevokedAt = &t
	}
	if used.Valid {
		t := time.UnixMilli(used.Int64).UTC()
		k.LastUsedAt = &t
	}
	k.Stacks = splitList(stacks)
	k.Hosts = splitList(hosts)
	k.Guest = guest == 1
	return k, nil
}

func splitList(raw string) []string {
	return cleanList(strings.Split(raw, ","))
}

func cleanList(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validRole(role string) bool {
	switch role {
	case "admin", "operator", "auditor", "viewer":
		return true
	default:
		return false
	}
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
