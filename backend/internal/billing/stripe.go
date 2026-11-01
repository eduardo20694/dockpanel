package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Stripe struct {
	Secret    string
	Webhook   string
	PriceUser string
	PriceSeat string
	HTTP      *http.Client
	APIBase   string
}

func StripeFromEnv() *Stripe {
	secret := firstEnv("STRIPE_SECRET_KEY", "DOCKERPILOT_STRIPE_SECRET_KEY")
	if secret == "" {
		return nil
	}
	return &Stripe{
		Secret:    secret,
		Webhook:   firstEnv("STRIPE_WEBHOOK_SECRET", "DOCKERPILOT_STRIPE_WEBHOOK_SECRET"),
		PriceUser: firstEnv("STRIPE_PRICE_INDIVIDUAL", "DOCKERPILOT_STRIPE_PRICE_INDIVIDUAL"),
		PriceSeat: firstEnv("STRIPE_PRICE_SEAT", "DOCKERPILOT_STRIPE_PRICE_SEAT"),
		HTTP:      &http.Client{Timeout: 20 * time.Second},
		APIBase:   "https://api.stripe.com",
	}
}

func (s *Stripe) Name() string { return "stripe" }

func (s *Stripe) CreateCheckout(ctx context.Context, req CheckoutRequest) (string, error) {
	if s == nil || s.Secret == "" {
		return "", ErrNotConfigured
	}
	price := s.PriceSeat
	qty := req.Seats
	mode := "subscription"
	if req.Plan == "individual" || qty <= 1 && req.Plan != "organization" {
		price = s.PriceUser
		qty = 1
		req.Plan = "individual"
	}
	if price == "" {
		return "", ErrNotConfigured
	}
	if qty < 1 {
		qty = 1
	}
	form := url.Values{}
	form.Set("mode", mode)
	form.Set("success_url", req.SuccessURL)
	form.Set("cancel_url", req.CancelURL)
	form.Set("client_reference_id", req.OrgID)
	form.Set("line_items[0][price]", price)
	form.Set("line_items[0][quantity]", strconv.Itoa(qty))
	form.Set("metadata[org_id]", req.OrgID)
	form.Set("metadata[plan]", req.Plan)
	form.Set("metadata[seats]", strconv.Itoa(qty))
	if req.Email != "" {
		form.Set("customer_email", req.Email)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.APIBase+"/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	httpReq.SetBasicAuth(s.Secret, "")
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := s.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("stripe %d: %s", resp.StatusCode, truncate(body))
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", errorsURL()
	}
	return out.URL, nil
}

func (s *Stripe) ParseWebhook(payload []byte, signature string) (Event, error) {
	if err := VerifyStripeSignature(payload, signature, s.Webhook, time.Now()); err != nil {
		return Event{}, err
	}
	var envelope struct {
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return Event{}, err
	}
	var obj map[string]any
	_ = json.Unmarshal(envelope.Data.Object, &obj)
	meta, _ := obj["metadata"].(map[string]any)
	ev := Event{
		Type:           envelope.Type,
		OrgID:          str(meta["org_id"]),
		Plan:           str(meta["plan"]),
		Seats:          atoi(str(meta["seats"])),
		CustomerID:     str(obj["customer"]),
		SubscriptionID: str(obj["subscription"]),
		Status:         "active",
	}
	if ev.OrgID == "" {
		ev.OrgID = str(obj["client_reference_id"])
	}
	switch envelope.Type {
	case "checkout.session.completed":
		if ev.Plan == "" {
			ev.Plan = "organization"
		}
		if ev.Seats < 1 {
			ev.Seats = 1
		}
	case "customer.subscription.deleted":
		ev.Status = "canceled"
		ev.Seats = 1
		ev.Plan = "individual"
	case "customer.subscription.updated":
		if st := str(obj["status"]); st == "past_due" || st == "canceled" || st == "unpaid" {
			ev.Status = "past_due"
		}
		if q := quantity(obj); q > 0 {
			ev.Seats = q
		}
	default:
		return Event{}, fmt.Errorf("evento ignorado: %s", envelope.Type)
	}
	return ev, nil
}

func quantity(obj map[string]any) int {
	items, _ := obj["items"].(map[string]any)
	data, _ := items["data"].([]any)
	if len(data) == 0 {
		return 0
	}
	first, _ := data[0].(map[string]any)
	switch q := first["quantity"].(type) {
	case float64:
		return int(q)
	case string:
		n, _ := strconv.Atoi(q)
		return n
	default:
		return 0
	}
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func truncate(b []byte) string {
	if len(b) > 300 {
		return string(b[:300])
	}
	return string(b)
}

func errorsURL() error { return fmt.Errorf("stripe não devolveu url") }
