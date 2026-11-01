// Package billing is the payment boundary. Stripe is one provider.
package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("cobrança não configurada")

type CheckoutRequest struct {
	OrgID      string
	Email      string
	Plan       string
	Seats      int
	SuccessURL string
	CancelURL  string
}

type Event struct {
	Type           string
	OrgID          string
	Plan           string
	Seats          int
	Status         string
	CustomerID     string
	SubscriptionID string
}

type Provider interface {
	Name() string
	CreateCheckout(ctx context.Context, req CheckoutRequest) (string, error)
	ParseWebhook(payload []byte, signature string) (Event, error)
}

// VerifyStripeSignature checks the Stripe-Signature header.
func VerifyStripeSignature(payload []byte, header, secret string, now time.Time) error {
	if secret == "" {
		return errors.New("webhook secret vazio")
	}
	var ts, v1 string
	for _, part := range strings.Split(header, ",") {
		k, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			ts = val
		case "v1":
			v1 = val
		}
	}
	if ts == "" || v1 == "" {
		return errors.New("assinatura ausente")
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return errors.New("timestamp inválido")
	}
	if now.Sub(time.Unix(sec, 0)) > 5*time.Minute || time.Unix(sec, 0).Sub(now) > time.Minute {
		return errors.New("timestamp fora da janela")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(payload)
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(v1)) {
		return errors.New("assinatura não confere")
	}
	return nil
}
