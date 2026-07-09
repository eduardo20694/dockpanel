package auth

import (
	"strings"
	"testing"
)

func TestResolveDatabaseURLFromParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_HOST", "127.0.0.1")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "dockpanel")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_DB", "dockpanel")
	t.Setenv("POSTGRES_SSLMODE", "disable")

	got := ResolveDatabaseURL()
	if got == "" || !strings.Contains(got, "127.0.0.1") || !strings.Contains(got, "dockpanel") {
		t.Fatalf("unexpected dsn: %q", got)
	}
}

func TestResolveDatabaseURLPrefersDATABASE_URL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://a:b@host:5432/db?sslmode=disable")
	t.Setenv("POSTGRES_HOST", "ignored")
	got := ResolveDatabaseURL()
	if got != "postgres://a:b@host:5432/db?sslmode=disable" {
		t.Fatalf("got %q", got)
	}
}

func TestDatabaseLabel(t *testing.T) {
	label := DatabaseLabel("postgres://dockpanel:xx@127.0.0.1:5432/dockpanel?sslmode=disable")
	if label != "dockpanel@127.0.0.1:5432/dockpanel" {
		t.Fatalf("got %q", label)
	}
}
