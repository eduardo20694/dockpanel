package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(b), "="), nil
}

func TOTPURI(secret, account string) string {
	return fmt.Sprintf("otpauth://totp/Dockpilot:%s?secret=%s&issuer=Dockpilot&digits=6&period=30", url.QueryEscape(account), secret)
}

func TOTPCode(secret string, at time.Time) (string, error) {
	raw := strings.ToUpper(strings.TrimSpace(secret))
	if m := len(raw) % 8; m != 0 {
		raw += strings.Repeat("=", 8-m)
	}
	key, err := base32.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", err
	}
	counter := uint64(at.Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[off])&0x7f)<<24 | uint32(sum[off+1])<<16 | uint32(sum[off+2])<<8 | uint32(sum[off+3])
	return fmt.Sprintf("%06d", bin%1000000), nil
}

func VerifyTOTP(secret, code string, at time.Time) bool {
	code = strings.TrimSpace(code)
	for _, delta := range []time.Duration{0, -30 * time.Second, 30 * time.Second} {
		got, err := TOTPCode(secret, at.Add(delta))
		if err == nil && hmac.Equal([]byte(got), []byte(code)) {
			return true
		}
	}
	return false
}
