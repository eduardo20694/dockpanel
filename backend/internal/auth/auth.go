package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	cookieName    = "dockpanel_token"
	defaultExpiry = 24 * time.Hour
)

var (
	ErrInvalidCredentials = errors.New("email ou senha inválidos")
	ErrAuthDisabled       = errors.New("autenticação desabilitada")
)

// User is the configured admin from DOCKPANEL_ADMIN_* env.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	Enabled  bool
	secret   []byte
	email    string
	password string
	name     string
}

// NewService builds env-based single-admin auth.
// Requires DOCKPANEL_ADMIN_EMAIL + DOCKPANEL_ADMIN_PASSWORD; otherwise Enabled=false.
func NewService() (*Service, error) {
	email := strings.TrimSpace(strings.ToLower(os.Getenv("DOCKPANEL_ADMIN_EMAIL")))
	pass := os.Getenv("DOCKPANEL_ADMIN_PASSWORD")
	if email == "" || pass == "" {
		return &Service{Enabled: false}, nil
	}
	secret := os.Getenv("DOCKPANEL_JWT_SECRET")
	if secret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		secret = base64.RawURLEncoding.EncodeToString(b)
		fmt.Println("aviso: DOCKPANEL_JWT_SECRET não definido — usando segredo efêmero (reiniciar invalida sessões)")
	}
	name := strings.TrimSpace(os.Getenv("DOCKPANEL_ADMIN_NAME"))
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
	return User{ID: "admin", Email: s.email, Name: s.name, Role: "admin"}
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
	return User{ID: c.UserID, Email: c.Email, Name: c.Name, Role: c.Role}
}

func CookieName() string     { return cookieName }
func TokenExpiry() time.Time { return time.Now().Add(defaultExpiry) }
