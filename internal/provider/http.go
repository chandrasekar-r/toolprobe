package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func errf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func postJSON(ctx context.Context, hc *http.Client, url string, headers map[string]string, payload any) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, errf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, errf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, errf("http: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, errf("read body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}

func statusErr(provider string, code int, body []byte) error {
	return errf("%s status %d: %s", provider, code, truncate(string(body), 500))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// normalizeArgs accepts a JSON object, array, or a string containing JSON.
func normalizeArgs(raw json.RawMessage) (string, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(raw) == "null" {
		return "{}", nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return "{}", nil
		}
		if !json.Valid([]byte(asString)) {
			return "", errf("tool arguments are not valid JSON: %s", truncate(asString, 200))
		}
		return asString, nil
	}
	if !json.Valid(raw) {
		return "", errf("tool arguments are not valid JSON")
	}
	return string(raw), nil
}
