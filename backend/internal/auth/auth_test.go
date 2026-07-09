package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("senha-forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "senha-forte-123") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestServiceDisabled(t *testing.T) {
	svc, err := NewService(nil)
	if err != nil {
		t.Fatal(err)
	}
	if svc.Enabled {
		t.Fatal("expected disabled without store")
	}
}
