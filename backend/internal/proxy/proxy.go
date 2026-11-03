// Package proxy guards raw Docker and Kubernetes calls used by the API and MCP.
package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"dockerpilot/internal/envx"

	"github.com/docker/docker/client"
)

const maxBody = 1 << 20

var (
	ErrBadPath         = errors.New("caminho rejeitado")
	ErrReadOnly        = errors.New("modo somente leitura: apenas GET e HEAD")
	ErrK8sUnconfigured = errors.New("Kubernetes não configurado neste Dockpilot (defina DOCKERPILOT_K8S_API e DOCKERPILOT_K8S_TOKEN)")
)

// ValidatePath rejects traversal, query strings and scheme-like paths.
func ValidatePath(p string) error {
	if p == "" || !strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
		return ErrBadPath
	}
	if strings.ContainsAny(p, "?#\\") || strings.Contains(p, "://") {
		return ErrBadPath
	}
	return nil
}

// ValidateMethod enforces the read-only policy. GET and HEAD always pass.
func ValidateMethod(method string, readOnly bool) error {
	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case http.MethodGet, http.MethodHead:
		return nil
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		if readOnly {
			return ErrReadOnly
		}
		return nil
	default:
		return ErrBadPath
	}
}

// DoDocker forwards one Engine API call through the client's HTTP transport.
func DoDocker(ctx context.Context, cli *client.Client, method, path string, body []byte, readOnly bool) (int, []byte, error) {
	if err := ValidatePath(path); err != nil {
		return 0, nil, err
	}
	if err := ValidateMethod(method, readOnly); err != nil {
		return 0, nil, err
	}
	if cli == nil {
		return 0, nil, errors.New("cliente Docker ausente")
	}
	if len(body) > maxBody {
		return 0, nil, errors.New("corpo grande demais")
	}
	version := cli.ClientVersion()
	if version == "" {
		version = "1.44"
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), "http://docker/v"+version+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cli.HTTPClient().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, raw, nil
}

// DoKubernetes forwards one request when DOCKERPILOT_K8S_API is set.
func DoKubernetes(ctx context.Context, method, path string, body []byte, readOnly bool) (int, []byte, error) {
	if err := ValidatePath(path); err != nil {
		return 0, nil, err
	}
	if err := ValidateMethod(method, readOnly); err != nil {
		return 0, nil, err
	}
	base := strings.TrimRight(envx.Get("K8S_API"), "/")
	if base == "" {
		return 0, nil, ErrK8sUnconfigured
	}
	if len(body) > maxBody {
		return 0, nil, errors.New("corpo grande demais")
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), base+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if tok := envx.GetRaw("K8S_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, raw, nil
}
