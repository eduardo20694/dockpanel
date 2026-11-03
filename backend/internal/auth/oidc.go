package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Preset names the built-in OIDC issuers.
func PresetIssuer(provider, tenant, base string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "google":
		return "https://accounts.google.com", nil
	case "entra", "microsoft":
		if tenant == "" {
			tenant = "common"
		}
		return "https://login.microsoftonline.com/" + tenant + "/v2.0", nil
	case "keycloak":
		if base == "" {
			return "", errors.New("informe a URL base do Keycloak")
		}
		realm := tenant
		if realm == "" {
			realm = "master"
		}
		return strings.TrimRight(base, "/") + "/realms/" + realm, nil
	case "", "generic":
		if base == "" {
			return "", errors.New("informe o issuer")
		}
		return strings.TrimRight(base, "/"), nil
	default:
		return "", errors.New("provedor desconhecido")
	}
}

type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	Redirect     string
	HTTP         *http.Client
}

func (o OIDC) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

type discovery struct {
	Auth   string `json:"authorization_endpoint"`
	Token  string `json:"token_endpoint"`
	JWKS   string `json:"jwks_uri"`
	Issuer string `json:"issuer"`
}

func (o OIDC) discover(ctx context.Context) (discovery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(o.Issuer, "/")+"/.well-known/openid-configuration", nil)
	if err != nil {
		return discovery{}, err
	}
	resp, err := o.client().Do(req)
	if err != nil {
		return discovery{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return discovery{}, fmt.Errorf("discovery %d", resp.StatusCode)
	}
	var doc discovery
	if err := json.Unmarshal(body, &doc); err != nil {
		return discovery{}, err
	}
	return doc, nil
}

func (o OIDC) AuthURL(ctx context.Context, state string) (string, error) {
	doc, err := o.discover(ctx)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("client_id", o.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("redirect_uri", o.Redirect)
	q.Set("state", state)
	return doc.Auth + "?" + q.Encode(), nil
}

func (o OIDC) Exchange(ctx context.Context, code string) (User, error) {
	doc, err := o.discover(ctx)
	if err != nil {
		return User{}, err
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", o.Redirect)
	form.Set("client_id", o.ClientID)
	form.Set("client_secret", o.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.Token, strings.NewReader(form.Encode()))
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.client().Do(req)
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return User{}, fmt.Errorf("token %d", resp.StatusCode)
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return User{}, err
	}
	return o.VerifyRS256(ctx, tok.IDToken, doc.JWKS)
}

func (o OIDC) VerifyRS256(ctx context.Context, idToken, jwksURL string) (User, error) {
	keys, err := o.fetchJWKS(ctx, jwksURL)
	if err != nil {
		return User{}, err
	}
	parsed, err := jwt.Parse(idToken, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, errors.New("algoritmo inesperado")
		}
		kid, _ := t.Header["kid"].(string)
		pub, ok := keys[kid]
		if !ok {
			for _, k := range keys {
				pub = k
				ok = true
				break
			}
		}
		if !ok {
			return nil, errors.New("jwks sem chave")
		}
		return pub, nil
	})
	if err != nil || !parsed.Valid {
		return User{}, errors.New("ID token inválido")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return User{}, errors.New("claims inválidas")
	}
	if iss, _ := claims["iss"].(string); iss != "" && o.Issuer != "" && iss != o.Issuer {
		return User{}, errors.New("issuer não confere")
	}
	email := firstClaim(claims, "email", "preferred_username", "upn")
	if email == "" {
		return User{}, errors.New("token sem email")
	}
	name := firstClaim(claims, "name")
	if name == "" {
		name = email
	}
	return User{
		ID: "sso:" + strings.ToLower(email), Email: strings.ToLower(email), Name: name,
		Role: mapRole(firstClaim(claims, "role"), claimGroups(claims["groups"])), OrgID: "default",
	}, nil
}

func (o OIDC) fetchJWKS(ctx context.Context, raw string) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	out := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "" && k.Kty != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}
		out[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}
	}
	if len(out) == 0 {
		return nil, errors.New("jwks vazio")
	}
	return out, nil
}
