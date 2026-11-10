// Package secrets stores stack and org secrets encrypted at rest.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dockerpilot/internal/envx"
)

var ErrNoKey = errors.New("defina DOCKERPILOT_SECRETS_KEY para usar o cofre")

type Meta struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"orgId"`
	Stack     string    `json:"stack"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Target    string    `json:"target"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type record struct {
	Meta
	Nonce string `json:"nonce"`
	Box   string `json:"box"`
}

type fileDoc struct {
	Items []record `json:"items"`
}

type Store struct {
	mu   sync.Mutex
	path string
	key  []byte
	doc  fileDoc
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "secrets.json")}
	if k, err := DeriveKey(envx.GetRaw("SECRETS_KEY")); err == nil {
		s.key = k
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.doc); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// DeriveKey accepts 32 raw bytes, standard base64, hex, or any passphrase (SHA-256).
func DeriveKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrNoKey
	}
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], nil
}

func (s *Store) Put(orgID, stack, name, kind, target, value string) (Meta, error) {
	if s == nil || len(s.key) != 32 {
		return Meta{}, ErrNoKey
	}
	orgID = clean(orgID, "default")
	stack = strings.TrimSpace(stack)
	name = strings.TrimSpace(name)
	kind = strings.ToLower(strings.TrimSpace(kind))
	target = strings.TrimSpace(target)
	if stack == "" || name == "" || value == "" {
		return Meta{}, errors.New("stack, nome e valor são obrigatórios")
	}
	if kind != "file" {
		kind = "env"
	}
	if target == "" {
		target = name
	}
	if kind == "file" && (strings.Contains(target, "..") || filepath.IsAbs(target)) {
		return Meta{}, errors.New("caminho de arquivo inválido")
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return Meta{}, err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return Meta{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Meta{}, err
	}
	aad := []byte(orgID + "|" + stack + "|" + name)
	box := gcm.Seal(nil, nonce, []byte(value), aad)
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := Meta{
		ID: newID(), OrgID: orgID, Stack: stack, Name: name, Kind: kind, Target: target, UpdatedAt: time.Now().UTC(),
	}
	rec := record{Meta: meta, Nonce: base64.StdEncoding.EncodeToString(nonce), Box: base64.StdEncoding.EncodeToString(box)}
	replaced := false
	for i := range s.doc.Items {
		if s.doc.Items[i].OrgID == orgID && s.doc.Items[i].Stack == stack && s.doc.Items[i].Name == name {
			rec.ID = s.doc.Items[i].ID
			rec.Meta.ID = rec.ID
			s.doc.Items[i] = rec
			replaced = true
			break
		}
	}
	if !replaced {
		s.doc.Items = append(s.doc.Items, rec)
	}
	if err := s.flush(); err != nil {
		return Meta{}, err
	}
	return rec.Meta, nil
}

func (s *Store) List(orgID, stack string) []Meta {
	if s == nil {
		return []Meta{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Meta
	for _, it := range s.doc.Items {
		if orgID != "" && it.OrgID != orgID {
			continue
		}
		if stack != "" && it.Stack != stack {
			continue
		}
		out = append(out, it.Meta)
	}
	if out == nil {
		out = []Meta{}
	}
	return out
}

func (s *Store) Delete(orgID, id string) error {
	if s == nil {
		return errors.New("cofre indisponível")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.doc.Items[:0]
	found := false
	for _, it := range s.doc.Items {
		if it.ID == id && (orgID == "" || it.OrgID == orgID) {
			found = true
			continue
		}
		next = append(next, it)
	}
	if !found {
		return errors.New("segredo não encontrado")
	}
	s.doc.Items = next
	return s.flush()
}

// Materialize decrypts env and file secrets for a deploy. Callers must audit the access and must not log values.
func (s *Store) Materialize(orgID, stack string) (map[string]string, map[string][]byte, error) {
	env := map[string]string{}
	files := map[string][]byte{}
	if s == nil {
		return env, files, nil
	}
	if len(s.key) != 32 {
		if len(s.List(orgID, stack)) > 0 {
			return nil, nil, ErrNoKey
		}
		return env, files, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	for _, it := range s.doc.Items {
		if it.OrgID != orgID || it.Stack != stack {
			continue
		}
		nonce, err := base64.StdEncoding.DecodeString(it.Nonce)
		if err != nil {
			return nil, nil, err
		}
		box, err := base64.StdEncoding.DecodeString(it.Box)
		if err != nil {
			return nil, nil, err
		}
		plain, err := gcm.Open(nil, nonce, box, []byte(it.OrgID+"|"+it.Stack+"|"+it.Name))
		if err != nil {
			return nil, nil, errors.New("não foi possível abrir o cofre com a chave mestra atual")
		}
		if it.Kind == "file" {
			files[it.Target] = plain
		} else {
			env[it.Target] = string(plain)
		}
	}
	return env, files, nil
}

func (s *Store) flush() error {
	raw, err := json.MarshalIndent(s.doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func clean(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func newID() string {
	b := make([]byte, 8)
	_, _ = io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}
