package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

func TestWebhookSignatureAndCheckoutEvent(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"type":"checkout.session.completed","data":{"object":{"client_reference_id":"default","customer":"cus_1","subscription":"sub_1","metadata":{"org_id":"default","plan":"organization","seats":"4"}}}}`)
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("."))
	mac.Write(payload)
	sig := "t=" + strconv.FormatInt(ts, 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
	if err := VerifyStripeSignature(payload, sig, secret, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := VerifyStripeSignature(payload, sig, "wrong", time.Now()); err == nil {
		t.Fatal("expected reject")
	}
	s := &Stripe{Webhook: secret}
	ev, err := s.ParseWebhook(payload, sig)
	if err != nil {
		t.Fatal(err)
	}
	if ev.OrgID != "default" || ev.Plan != "organization" || ev.Seats != 4 || ev.Status != "active" {
		t.Fatalf("%+v", ev)
	}
}
