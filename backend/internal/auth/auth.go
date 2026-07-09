package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	cookieName     = "dockpanel_token"
	defaultExpiry  = 7 * 24 * time.Hour
	bcryptCost     = 12
)

var (
	ErrInvalidCredentials = errors.New("email ou senha inválidos")
	ErrAuthDisabled       = errors.New("autenticação desabilitada")
)

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
	Enabled bool
	secret  []byte
	store   *Store
}

func NewService(store *Store) (*Service, error) {
	if store == nil {
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
	return &Service{Enabled: true, secret: []byte(secret), store: store}, nil
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(b), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (s *Service) Login(email, password string) (token string, user User, err error) {
	if !s.Enabled {
		return "", User{}, ErrAuthDisabled
	}
	email = strings.TrimSpace(strings.ToLower(email))
	u, hash, err := s.store.FindByEmail(email)
	if err != nil {
		return "", User{}, ErrInvalidCredentials
	}
	if !CheckPassword(hash, password) {
		return "", User{}, ErrInvalidCredentials
	}
	token, err = s.sign(u)
	return token, u, err
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

func CookieName() string { return cookieName }

func TokenExpiry() time.Time { return time.Now().Add(defaultExpiry) }
