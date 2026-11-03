// Package org stores organizations, users and teams for multi-tenant Dockpilot.
// The env admin is not written here; it is the break-glass account owned by auth.
package org

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const fileName = "orgs.json"

var (
	ErrNotFound    = errors.New("não encontrado")
	ErrExists      = errors.New("já existe")
	ErrInvalid     = errors.New("dados inválidos")
	ErrBadPassword = errors.New("email ou senha inválidos")
)

// Org is a tenant.
type Org struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"createdAt"`
}

// User is a directory account. PasswordHash is never returned by the API layer.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	OrgID        string    `json:"orgId"`
	PasswordHash string    `json:"passwordHash,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Team groups users inside an organization.
type Team struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"orgId"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// Membership is a user on a team. Role overrides the user role inside that team
// when it is more specific; the account role remains the API permission.
type Membership struct {
	TeamID string `json:"teamId"`
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

type document struct {
	Orgs        []Org        `json:"orgs"`
	Users       []User       `json:"users"`
	Teams       []Team       `json:"teams"`
	Memberships []Membership `json:"memberships"`
}

type Store struct {
	mu   sync.Mutex
	path string
	doc  document
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, fileName)}
	raw, err := os.ReadFile(s.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.doc); err != nil {
			return nil, err
		}
	}
	if len(s.doc.Orgs) == 0 {
		s.doc.Orgs = []Org{{
			ID: "default", Name: "Organização principal", Slug: "default", CreatedAt: time.Now().UTC(),
		}}
		if err := s.flush(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Orgs() []Org {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Org(nil), s.doc.Orgs...)
	return out
}

func (s *Store) Users(orgID string) []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []User
	for _, u := range s.doc.Users {
		if orgID == "" || u.OrgID == orgID {
			cp := u
			cp.PasswordHash = ""
			out = append(out, cp)
		}
	}
	if out == nil {
		out = []User{}
	}
	return out
}

func (s *Store) Teams(orgID string) []Team {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Team
	for _, t := range s.doc.Teams {
		if orgID == "" || t.OrgID == orgID {
			out = append(out, t)
		}
	}
	if out == nil {
		out = []Team{}
	}
	return out
}

func (s *Store) Memberships(teamID string) []Membership {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Membership
	for _, m := range s.doc.Memberships {
		if teamID == "" || m.TeamID == teamID {
			out = append(out, m)
		}
	}
	if out == nil {
		out = []Membership{}
	}
	return out
}

// CreateUser adds a password account. Role must be admin, operator, auditor or viewer.
func (s *Store) CreateUser(email, name, password, role, orgID string) (User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)
	role = strings.ToLower(strings.TrimSpace(role))
	if orgID == "" {
		orgID = "default"
	}
	if email == "" || !strings.Contains(email, "@") || len(password) < 8 {
		return User{}, ErrInvalid
	}
	if !validRole(role) {
		return User{}, ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasOrg(orgID) {
		return User{}, ErrNotFound
	}
	for _, u := range s.doc.Users {
		if u.Email == email {
			return User{}, ErrExists
		}
	}
	u := User{
		ID: newID(), Email: email, Name: name, Role: role, OrgID: orgID,
		PasswordHash: string(hash), CreatedAt: time.Now().UTC(),
	}
	if u.Name == "" {
		u.Name = email
	}
	s.doc.Users = append(s.doc.Users, u)
	if err := s.flush(); err != nil {
		return User{}, err
	}
	u.PasswordHash = ""
	return u, nil
}

// UpdateUser changes name and/or role.
func (s *Store) UpdateUser(id, name, role string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.doc.Users {
		if s.doc.Users[i].ID != id {
			continue
		}
		if strings.TrimSpace(name) != "" {
			s.doc.Users[i].Name = strings.TrimSpace(name)
		}
		if role != "" {
			role = strings.ToLower(role)
			if !validRole(role) {
				return User{}, ErrInvalid
			}
			s.doc.Users[i].Role = role
		}
		if err := s.flush(); err != nil {
			return User{}, err
		}
		u := s.doc.Users[i]
		u.PasswordHash = ""
		return u, nil
	}
	return User{}, ErrNotFound
}

func (s *Store) CreateTeam(orgID, name string) (Team, error) {
	name = strings.TrimSpace(name)
	if orgID == "" {
		orgID = "default"
	}
	if name == "" {
		return Team{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasOrg(orgID) {
		return Team{}, ErrNotFound
	}
	t := Team{ID: newID(), OrgID: orgID, Name: name, CreatedAt: time.Now().UTC()}
	s.doc.Teams = append(s.doc.Teams, t)
	if err := s.flush(); err != nil {
		return Team{}, err
	}
	return t, nil
}

func (s *Store) AddMember(teamID, userID, role string) (Membership, error) {
	if role == "" {
		role = "operator"
	}
	role = strings.ToLower(role)
	if !validRole(role) {
		return Membership{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasTeam(teamID) || !s.hasUser(userID) {
		return Membership{}, ErrNotFound
	}
	for i := range s.doc.Memberships {
		if s.doc.Memberships[i].TeamID == teamID && s.doc.Memberships[i].UserID == userID {
			s.doc.Memberships[i].Role = role
			if err := s.flush(); err != nil {
				return Membership{}, err
			}
			return s.doc.Memberships[i], nil
		}
	}
	m := Membership{TeamID: teamID, UserID: userID, Role: role}
	s.doc.Memberships = append(s.doc.Memberships, m)
	if err := s.flush(); err != nil {
		return Membership{}, err
	}
	return m, nil
}

// Authenticate checks directory users. ok is false when the email is unknown
// so the caller can keep a generic error.
func (s *Store) Authenticate(email, password string) (User, bool, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.doc.Users {
		if u.Email != email {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
			return User{}, true, ErrBadPassword
		}
		u.PasswordHash = ""
		return u, true, nil
	}
	return User{}, false, nil
}

func (s *Store) hasOrg(id string) bool {
	for _, o := range s.doc.Orgs {
		if o.ID == id {
			return true
		}
	}
	return false
}

func (s *Store) hasTeam(id string) bool {
	for _, t := range s.doc.Teams {
		if t.ID == id {
			return true
		}
	}
	return false
}

func (s *Store) hasUser(id string) bool {
	for _, u := range s.doc.Users {
		if u.ID == id {
			return true
		}
	}
	return false
}

func validRole(role string) bool {
	switch role {
	case "admin", "operator", "auditor", "viewer":
		return true
	default:
		return false
	}
}

func (s *Store) flush() error {
	raw, err := json.MarshalIndent(s.doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
