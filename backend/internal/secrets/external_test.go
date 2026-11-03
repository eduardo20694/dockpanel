package secrets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExternalBackends(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/db", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "tok" {
			http.Error(w, "no", 401)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"data":{"password":"vault-secret"}}}`))
	})
	mux.HandleFunc("/v1/vaults/ops/items/db", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer op" {
			http.Error(w, "no", 401)
			return
		}
		_, _ = w.Write([]byte(`{"fields":[{"label":"password","value":"op-secret"}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	v, err := (Vault{Addr: srv.URL, Token: "tok", HTTP: srv.Client()}).Get(context.Background(), "db")
	if err != nil || v != "vault-secret" {
		t.Fatal(v, err)
	}
	op, err := (OnePassword{URL: srv.URL, Token: "op", Vault: "ops", HTTP: srv.Client()}).Get(context.Background(), "db")
	if err != nil || op != "op-secret" {
		t.Fatal(op, err)
	}
}
