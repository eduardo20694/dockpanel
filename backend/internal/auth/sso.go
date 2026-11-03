package auth

import (
	"errors"
	"strings"

	"dockerpilot/internal/envx"

	"github.com/golang-jwt/jwt/v5"
)

// SSOInfo is the public configuration for the login screen.
type SSOInfo struct {
	Enabled  bool   `json:"enabled"`
	Issuer   string `json:"issuer,omitempty"`
	ClientID string `json:"clientId,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

// SSOConfig reports whether an external identity provider is configured.
// Mode "hmac" is the built-in verifier (tests and simple shared-secret IdPs).
// Mode "oidc" means an issuer is set and tokens are expected to be HS256 with
// DOCKERPILOT_OIDC_HMAC_SECRET, or the deployment documents a JWKS verifier later.
func SSOConfig() SSOInfo {
	info := SSOInfo{
		Issuer:   envx.Get("OIDC_ISSUER"),
		ClientID: envx.Get("OIDC_CLIENT_ID"),
	}
	info.Enabled = info.Issuer != "" && info.ClientID != ""
	if envx.GetRaw("OIDC_HMAC_SECRET") != "" {
		info.Mode = "hmac"
	} else if info.Enabled {
		info.Mode = "oidc"
	}
	return info
}

// ExchangeSSO verifies an ID token and maps it to a Dockpilot user.
// HS256 with DOCKERPILOT_OIDC_HMAC_SECRET is implemented so organizations can
// plug a gateway that signs tokens without bundling a JWKS client.
func (s *Service) ExchangeSSO(idToken string) (User, error) {
	if s == nil || !s.Enabled {
		return User{}, ErrAuthDisabled
	}
	cfg := SSOConfig()
	if !cfg.Enabled {
		return User{}, errors.New("SSO não configurado")
	}
	secret := envx.GetRaw("OIDC_HMAC_SECRET")
	if secret == "" {
		return User{}, errors.New("defina DOCKERPILOT_OIDC_HMAC_SECRET para validar o ID token (SSO pronto para o gateway da organização)")
	}
	parsed, err := jwt.Parse(idToken, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("algoritmo inesperado")
		}
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		return User{}, errors.New("ID token inválido")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return User{}, errors.New("claims inválidas")
	}
	if iss, _ := claims["iss"].(string); iss != "" && iss != cfg.Issuer {
		return User{}, errors.New("issuer não confere")
	}
	if aud := claimString(claims["aud"]); aud != "" && aud != cfg.ClientID && !strings.Contains(aud, cfg.ClientID) {
		return User{}, errors.New("audience não confere")
	}
	email := firstClaim(claims, "email", "preferred_username", "upn")
	if email == "" {
		return User{}, errors.New("token sem email")
	}
	name := firstClaim(claims, "name")
	if name == "" {
		name = email
	}
	role := mapRole(firstClaim(claims, "role"), claimGroups(claims["groups"]))
	return User{
		ID:    "sso:" + strings.ToLower(email),
		Email: strings.ToLower(email),
		Name:  name,
		Role:  role,
		OrgID: "default",
	}, nil
}

func mapRole(role string, groups []string) string {
	adminGroup := envx.Get("OIDC_ADMIN_GROUP")
	if adminGroup == "" {
		adminGroup = "dockpilot-admins"
	}
	opGroup := envx.Get("OIDC_OPERATOR_GROUP")
	if opGroup == "" {
		opGroup = "dockpilot-operators"
	}
	switch strings.ToLower(role) {
	case "admin", "operator", "auditor", "viewer":
		return strings.ToLower(role)
	}
	for _, g := range groups {
		switch g {
		case adminGroup:
			return "admin"
		case opGroup:
			return "operator"
		}
	}
	return "viewer"
}

func firstClaim(c jwt.MapClaims, keys ...string) string {
	for _, k := range keys {
		if s, ok := c[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func claimString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		var parts []string
		for _, p := range t {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func claimGroups(v interface{}) []string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return strings.Split(t, ",")
	case []interface{}:
		var out []string
		for _, p := range t {
			if s, ok := p.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
