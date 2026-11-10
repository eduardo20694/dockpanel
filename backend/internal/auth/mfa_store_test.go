package auth

import (
	"testing"
	"time"
)

func TestMFARequiresCode(t *testing.T) {
	s, err := OpenMFA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if s.HasTOTP("ana@org.dev") || !s.CheckTOTP("ana@org.dev", "", time.Now()) {
		t.Fatal("empty store should not demand a code")
	}
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0).UTC()
	code, err := TOTPCode(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmTOTP("ana@org.dev", secret, "000000", now); err == nil && code != "000000" {
		t.Fatal("bad confirm accepted")
	}
	if err := s.ConfirmTOTP("ana@org.dev", secret, code, now); err != nil {
		t.Fatal(err)
	}
	if !s.HasTOTP("ana@org.dev") || !s.CheckTOTP("ana@org.dev", code, now) || s.CheckTOTP("ana@org.dev", "000000", now) {
		t.Fatal("check")
	}
}
