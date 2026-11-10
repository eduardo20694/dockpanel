package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type factor struct {
	Secret   string    `json:"secret,omitempty"`
	Passkeys []Passkey `json:"passkeys,omitempty"`
}

type MFAStore struct {
	mu   sync.Mutex
	path string
	docs map[string]factor
}

func OpenMFA(dir string) (*MFAStore, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &MFAStore{path: filepath.Join(dir, "mfa.json"), docs: map[string]factor{}}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &s.docs)
	}
	if s.docs == nil {
		s.docs = map[string]factor{}
	}
	return s, nil
}

func (s *MFAStore) HasTOTP(email string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.docs[email].Secret != ""
}

func (s *MFAStore) ConfirmTOTP(email, secret, code string, at time.Time) error {
	if !VerifyTOTP(secret, code, at) {
		return errMFA
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.docs[email]
	cur.Secret = secret
	s.docs[email] = cur
	return s.flush()
}

func (s *MFAStore) CheckTOTP(email, code string, at time.Time) bool {
	if s == nil || !s.HasTOTP(email) {
		return true
	}
	s.mu.Lock()
	secret := s.docs[email].Secret
	s.mu.Unlock()
	return VerifyTOTP(secret, code, at)
}

func (s *MFAStore) AddPasskey(email string, key Passkey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.docs[email]
	cur.Passkeys = append(cur.Passkeys, key)
	s.docs[email] = cur
	return s.flush()
}

func (s *MFAStore) Passkeys(email string) []Passkey {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Passkey{}, s.docs[email].Passkeys...)
}

func (s *MFAStore) flush() error {
	raw, err := json.MarshalIndent(s.docs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

var errMFA = mfaError("código TOTP inválido")

type mfaError string

func (e mfaError) Error() string { return string(e) }
