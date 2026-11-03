package auth

import (
	"testing"
	"time"
)

func TestTOTPRoundTrip(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0).UTC()
	code, err := TOTPCode(secret, now)
	if err != nil || len(code) != 6 {
		t.Fatal(code, err)
	}
	if !VerifyTOTP(secret, code, now) {
		t.Fatal("verify")
	}
	if VerifyTOTP(secret, "000000", now) && code != "000000" {
		t.Fatal("accepted wrong code")
	}
	if !stringsContains(TOTPURI(secret, "ana@org.dev"), "otpauth://totp/") {
		t.Fatal(TOTPURI(secret, "ana@org.dev"))
	}
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (func() bool { return contains(s, sub) })()))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
