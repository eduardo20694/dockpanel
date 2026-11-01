package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"dockerpilot/internal/apikey"
	"dockerpilot/internal/auth"
	"dockerpilot/internal/billing"
	"dockerpilot/internal/org"
	"dockerpilot/internal/seats"
)

type fakeBilling struct{}

func (fakeBilling) Name() string { return "fake" }
func (fakeBilling) CreateCheckout(ctx context.Context, req billing.CheckoutRequest) (string, error) {
	return "https://checkout.test/" + req.Plan + "/" + strconv.Itoa(req.Seats), nil
}
func (fakeBilling) ParseWebhook(payload []byte, signature string) (billing.Event, error) {
	if signature != "ok" {
		return billing.Event{}, billing.ErrNotConfigured
	}
	return billing.Event{Type: "checkout.session.completed", OrgID: "default", Plan: "organization", Seats: 5, Status: "active"}, nil
}

func TestBillingAndKeysFlow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKERPILOT_ADMIN_EMAIL", "admin@dockpilot.dev")
	t.Setenv("DOCKERPILOT_ADMIN_PASSWORD", "secret-pass")
	t.Setenv("DOCKERPILOT_JWT_SECRET", "unit-test-secret-key-32-characters!")
	svc, err := auth.NewService()
	if err != nil {
		t.Fatal(err)
	}
	orgs, err := org.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := apikey.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	seatStore, err := seats.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := NewServerFromDeps(Deps{Auth: svc, Orgs: orgs, Keys: keys, Seats: seatStore, Billing: fakeBilling{}, DataDir: dir})
	login := do(s, http.MethodPost, "/api/auth/login", `{"email":"admin@dockpilot.dev","password":"secret-pass"}`, "")
	var tok string
	for _, c := range login.Result().Cookies() {
		if c.Name == "dockerpilot_token" {
			tok = c.Value
		}
	}
	if tok == "" {
		t.Fatal(login.Body.String())
	}
	blocked := do(s, http.MethodPost, "/api/users", `{"email":"a@b.c","name":"A","password":"senha-forte","role":"viewer"}`, tok)
	if blocked.Code != http.StatusPaymentRequired {
		t.Fatalf("seat %d %s", blocked.Code, blocked.Body.String())
	}
	wh := httptest.NewRequest(http.MethodPost, "/api/billing/webhook", strings.NewReader(`{}`))
	wh.Header.Set("Stripe-Signature", "ok")
	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, wh)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"seats":5`) {
		t.Fatalf("webhook %d %s", rr.Code, rr.Body.String())
	}
	created := do(s, http.MethodPost, "/api/api-keys", `{"name":"claude","role":"operator"}`, tok)
	if created.Code != 200 || !strings.Contains(created.Body.String(), "dp_live_") {
		t.Fatalf("key %d %s", created.Code, created.Body.String())
	}
	check := do(s, http.MethodPost, "/api/billing/checkout", `{"plan":"organization","seats":5}`, tok)
	if check.Code != 200 || !strings.Contains(check.Body.String(), "https://checkout.test/organization/5") {
		t.Fatalf("checkout %d %s", check.Code, check.Body.String())
	}
}
