package auth

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ResolveDatabaseURL monta a DSN a partir de DATABASE_URL ou variáveis POSTGRES_*.
func ResolveDatabaseURL() string {
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		return dsn
	}
	host := strings.TrimSpace(os.Getenv("POSTGRES_HOST"))
	user := strings.TrimSpace(os.Getenv("POSTGRES_USER"))
	password := os.Getenv("POSTGRES_PASSWORD")
	db := strings.TrimSpace(os.Getenv("POSTGRES_DB"))
	if host == "" || user == "" || db == "" {
		return ""
	}
	port := strings.TrimSpace(os.Getenv("POSTGRES_PORT"))
	if port == "" {
		port = "5432"
	}
	ssl := strings.TrimSpace(os.Getenv("POSTGRES_SSLMODE"))
	if ssl == "" {
		ssl = "disable"
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   db,
	}
	q := u.Query()
	q.Set("sslmode", ssl)
	u.RawQuery = q.Encode()
	return u.String()
}

// DatabaseLabel devolve host/db para log sem expor senha.
func DatabaseLabel(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "postgres"
	}
	db := strings.TrimPrefix(u.Path, "/")
	if db == "" {
		db = "?"
	}
	return fmt.Sprintf("%s@%s/%s", u.User.Username(), u.Host, db)
}
