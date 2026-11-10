package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// External is a remote secret backend. Values are for injection, never for the UI.
type External interface {
	Get(ctx context.Context, ref string) (string, error)
}

type Vault struct {
	Addr  string
	Token string
	Mount string
	HTTP  *http.Client
}

func (v Vault) client() *http.Client {
	if v.HTTP != nil {
		return v.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (v Vault) Get(ctx context.Context, ref string) (string, error) {
	mount := v.Mount
	if mount == "" {
		mount = "secret"
	}
	u := strings.TrimRight(v.Addr, "/") + "/v1/" + mount + "/data/" + strings.TrimLeft(ref, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", v.Token)
	resp, err := v.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("vault %d", resp.StatusCode)
	}
	var doc struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if len(doc.Data.Data) == 0 {
		return "", fmt.Errorf("vault sem valor")
	}
	for _, val := range doc.Data.Data {
		return val, nil
	}
	return "", fmt.Errorf("vault sem valor")
}

type OnePassword struct {
	URL   string
	Token string
	Vault string
	HTTP  *http.Client
}

func (o OnePassword) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (o OnePassword) Get(ctx context.Context, item string) (string, error) {
	u := strings.TrimRight(o.URL, "/") + "/v1/vaults/" + url.PathEscape(o.Vault) + "/items/" + url.PathEscape(item)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+o.Token)
	resp, err := o.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("1password %d", resp.StatusCode)
	}
	var doc struct {
		Fields []struct {
			Value string `json:"value"`
			Label string `json:"label"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	for _, f := range doc.Fields {
		if f.Label == "password" || f.Label == "credential" || f.Value != "" {
			return f.Value, nil
		}
	}
	return "", fmt.Errorf("1password sem campo")
}
