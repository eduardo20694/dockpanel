package updates

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

type Digest struct {
	Digest   string
	NewerTag string
}

type Registry interface {
	Latest(ctx context.Context, image, user, pass, token string) (Digest, error)
}

type HTTPRegistry struct {
	Client *http.Client
}

func (h *HTTPRegistry) client() *http.Client {
	if h != nil && h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (h *HTTPRegistry) Latest(ctx context.Context, image, user, pass, token string) (Digest, error) {
	ref := Parse(image)
	reg := registryHost(ref.Host)
	tok, err := h.token(ctx, ref, reg, user, pass, token)
	if err != nil {
		return Digest{}, err
	}
	digest, err := h.manifest(ctx, reg, ref.Repo, ref.Tag, tok, user, pass)
	if err != nil {
		return Digest{}, err
	}
	tags, _ := h.tags(ctx, reg, ref.Repo, tok, user, pass)
	return Digest{Digest: digest, NewerTag: newerTag(ref.Tag, tags)}, nil
}

func registryHost(host string) string {
	if host == "docker.io" {
		return "registry-1.docker.io"
	}
	return host
}

func (h *HTTPRegistry) token(ctx context.Context, ref Ref, registry, user, pass, token string) (string, error) {
	if token != "" {
		return token, nil
	}
	authURL := "https://" + registry + "/v2/"
	if ref.Host == "docker.io" {
		q := url.Values{}
		q.Set("service", "registry.docker.io")
		q.Set("scope", "repository:"+ref.Repo+":pull")
		authURL = "https://auth.docker.io/token?" + q.Encode()
		return h.fetchToken(ctx, authURL, user, pass)
	}
	if ref.Host == "ghcr.io" {
		q := url.Values{}
		q.Set("service", "ghcr.io")
		q.Set("scope", "repository:"+ref.Repo+":pull")
		return h.fetchToken(ctx, "https://ghcr.io/token?"+q.Encode(), user, pass)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+registry+"/v2/", nil)
	if err != nil {
		return "", err
	}
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return "", err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return "", nil
	}
	chal := resp.Header.Get("Www-Authenticate")
	realm, service, scope := parseBearer(chal)
	if realm == "" {
		return "", nil
	}
	q := url.Values{}
	if service != "" {
		q.Set("service", service)
	}
	if scope != "" {
		q.Set("scope", scope)
	} else {
		q.Set("scope", "repository:"+ref.Repo+":pull")
	}
	return h.fetchToken(ctx, realm+"?"+q.Encode(), user, pass)
}

func (h *HTTPRegistry) fetchToken(ctx context.Context, raw, user, pass string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", err
	}
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("token %d", resp.StatusCode)
	}
	var doc struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if doc.Token != "" {
		return doc.Token, nil
	}
	return doc.AccessToken, nil
}

func (h *HTTPRegistry) manifest(ctx context.Context, host, repo, tag, token, user, pass string) (string, error) {
	u := "https://" + host + "/v2/" + repo + "/manifests/" + tag
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
	}, ", "))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("manifest %d", resp.StatusCode)
	}
	d := resp.Header.Get("Docker-Content-Digest")
	if d == "" {
		return "", fmt.Errorf("registry não devolveu digest")
	}
	return d, nil
}

func (h *HTTPRegistry) tags(ctx context.Context, host, repo, token, user, pass string) ([]string, error) {
	u := "https://" + host + "/v2/" + repo + "/tags/list"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tags %d", resp.StatusCode)
	}
	var doc struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	return doc.Tags, nil
}

func parseBearer(h string) (realm, service, scope string) {
	h = strings.TrimSpace(h)
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return "", "", ""
	}
	h = h[7:]
	for _, part := range strings.Split(h, ",") {
		part = strings.TrimSpace(part)
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		switch strings.ToLower(k) {
		case "realm":
			realm = v
		case "service":
			service = v
		case "scope":
			scope = v
		}
	}
	return realm, service, scope
}
