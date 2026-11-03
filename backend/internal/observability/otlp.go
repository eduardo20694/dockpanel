package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// OTLPEndpoint is optional. Empty means metrics stay on /metrics only.
func OTLPEndpoint() string {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
}

// PushMetrics sends one OTLP/JSON gauge export. No-op when the endpoint is unset.
func PushMetrics(ctx context.Context, client *http.Client, name string, value int) error {
	ep := OTLPEndpoint()
	if ep == "" {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	payload := map[string]any{
		"resourceMetrics": []any{map[string]any{
			"scopeMetrics": []any{map[string]any{
				"metrics": []any{map[string]any{
					"name": name,
					"gauge": map[string]any{
						"dataPoints": []any{map[string]any{"asInt": fmt.Sprint(value), "timeUnixNano": fmt.Sprint(time.Now().UnixNano())}},
					},
				}},
			}},
		}},
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep+"/v1/metrics", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("otlp %d", resp.StatusCode)
	}
	return nil
}
