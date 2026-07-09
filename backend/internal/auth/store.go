package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context) (*Store, error) {
	dsn := ResolveDatabaseURL()
	if dsn == "" {
		return nil, nil
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping (%s): %w", DatabaseLabel(dsn), err)
	}
	fmt.Printf("auth: conectado ao PostgreSQL (%s)\n", DatabaseLabel(dsn))
	st := &Store{pool: pool}
	if err := st.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := st.bootstrapAdmin(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'admin',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	if err != nil && !isInsufficientPrivilege(err) {
		return err
	}
	_, err = s.pool.Exec(ctx, `
CREATE INDEX IF NOT EXISTS idx_users_email ON users (lower(email))`)
	if err != nil && !isInsufficientPrivilege(err) {
		return err
	}
	if err != nil && isInsufficientPrivilege(err) {
		fmt.Println("auth: aviso — tabela users existe mas dockpanel não é owner; rode: ALTER TABLE users OWNER TO dockpanel;")
	}
	return nil
}

func isInsufficientPrivilege(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "42501"
}

func (s *Store) bootstrapAdmin(ctx context.Context) error {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	email := strings.TrimSpace(strings.ToLower(os.Getenv("DOCKPANEL_ADMIN_EMAIL")))
	password := os.Getenv("DOCKPANEL_ADMIN_PASSWORD")
	name := os.Getenv("DOCKPANEL_ADMIN_NAME")
	if name == "" {
		name = "Administrador"
	}
	if email == "" || password == "" {
		return fmt.Errorf("nenhum usuário no banco — defina DOCKPANEL_ADMIN_EMAIL e DOCKPANEL_ADMIN_PASSWORD")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO users (id, email, password_hash, name, role, created_at)
VALUES ($1, $2, $3, $4, 'admin', $5)`,
		uuid.NewString(), email, hash, name, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("auth: usuário admin criado (%s)\n", email)
	return nil
}

func (s *Store) FindByEmail(email string) (User, string, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(context.Background(), `
SELECT id, email, name, role, password_hash FROM users WHERE lower(email) = lower($1)`,
		email).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &hash)
	return u, hash, err
}

func (s *Store) FindByID(id string) (User, error) {
	var u User
	err := s.pool.QueryRow(context.Background(), `
SELECT id, email, name, role FROM users WHERE id = $1`, id).Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	return u, err
}
