package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"dockerpilot/internal/envx"

	"github.com/golang-jwt/jwt/v5"
)

const (
	cookieName       = "dockerpilot_token"
	legacyCookieName = "dockpanel_token"
	defaultExpiry    = 24 * time.Hour
)

var (
	ErrInvalidCredentials = errors.New("email ou senha inválidos")
	ErrAuthDisabled       = errors.New("autenticação desabilitada")
)

// User is an authenticated principal (env admin, directory user, or SSO).
type User struct {
	ID     string   `json:"id"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Role   string   `json:"role"`
	OrgID  string   `json:"orgId,omitempty"`
	KeyID  string   `json:"-"`
	Stacks []string `json:"-"`
	Hosts  []string `json:"-"`
	Guest  bool     `json:"-"`
}

type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	OrgID  string `json:"org,omitempty"`
	jwt.RegisteredClaims
}

type Service struct {
	Enabled  bool
	secret   []byte
	email    string
	password string
	name     string
}

// NewService builds env-based break-glass admin auth.
// Requires DOCKERPILOT_ADMIN_EMAIL + DOCKERPILOT_ADMIN_PASSWORD
// (DOCKPANEL_* remains accepted). Otherwise Enabled=false.
func NewService() (*Service, error) {
	email := strings.TrimSpace(strings.ToLower(envx.Get("ADMIN_EMAIL")))
	pass := envx.GetRaw("ADMIN_PASSWORD")
	if email == "" || pass == "" {
		return &Service{Enabled: false}, nil
	}
	secret := envx.GetRaw("JWT_SECRET")
	if secret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		secret = base64.RawURLEncoding.EncodeToString(b)
		fmt.Println("aviso: DOCKERPILOT_JWT_SECRET não definido — usando segredo efêmero (reiniciar invalida sessões)")
	}
	name := strings.TrimSpace(envx.Get("ADMIN_NAME"))
	if name == "" {
		name = "Administrador"
	}
	return &Service{
		Enabled:  true,
		secret:   []byte(secret),
		email:    email,
		password: pass,
		name:     name,
	}, nil
}

func (s *Service) AdminUser() User {
	return User{ID: "admin", Email: s.email, Name: s.name, Role: "admin", OrgID: "default"}
}

// Issue signs a token for a directory or SSO user. The service must be enabled
// so a stable JWT secret exists.
func (s *Service) Issue(u User) (string, error) {
	if !s.Enabled {
		return "", ErrAuthDisabled
	}
	if u.Role == "" {
		u.Role = "viewer"
	}
	if u.OrgID == "" {
		u.OrgID = "default"
	}
	return s.sign(u)
}

type LoginResult struct {
	AccessToken string
	User        User
}

func (s *Service) Login(email, password string) (LoginResult, error) {
	if !s.Enabled {
		return LoginResult{}, ErrAuthDisabled
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if subtle.ConstantTimeCompare([]byte(email), []byte(s.email)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(s.password)) != 1 {
		return LoginResult{}, ErrInvalidCredentials
	}
	u := s.AdminUser()
	access, err := s.sign(u)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{AccessToken: access, User: u}, nil
}

func (s *Service) sign(u User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: u.ID,
		Email:  u.Email,
		Name:   u.Name,
		Role:   u.Role,
		OrgID:  u.OrgID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(defaultExpiry)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(s.secret)
}

func (s *Service) ParseToken(token string) (*Claims, error) {
	if !s.Enabled || token == "" {
		return nil, errors.New("unauthorized")
	}
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (s *Service) UserFromClaims(c *Claims) User {
	return User{ID: c.UserID, Email: c.Email, Name: c.Name, Role: c.Role, OrgID: c.OrgID}
}

func CookieName() string       { return cookieName }
func LegacyCookieName() string { return legacyCookieName }
func TokenExpiry() time.Time   { return time.Now().Add(defaultExpiry) }
