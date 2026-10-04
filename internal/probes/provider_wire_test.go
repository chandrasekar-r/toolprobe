package probes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chandrasekar-r/toolprobe/internal/provider"
)

// TestDefaultProbesNativeProviders runs every checked-in probe against a
// recorded native response for each provider. The fixture is built from
// probe.mock so the assertions match the baseline, while the HTTP body is
// the provider's own JSON.
func TestDefaultProbesNativeProviders(t *testing.T) {
	ps, err := LoadDir("../../probes/default")
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 17 {
		t.Fatalf("expected 17 default probes, got %d", len(ps))
	}
	providers := []string{provider.OpenAI, provider.Anthropic, provider.Gemini, provider.Cloudflare}
	for _, name := range providers {
		t.Run(name, func(t *testing.T) {
			for _, probe := range ps {
				probe := probe
				t.Run(probe.Name, func(t *testing.T) {
					var wireErr string
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						raw, err := io.ReadAll(r.Body)
						if err != nil {
							wireErr = err.Error()
							http.Error(w, wireErr, 500)
							return
						}
						var gotBody map[string]any
						if err := json.Unmarshal(raw, &gotBody); err != nil {
							wireErr = err.Error()
							http.Error(w, wireErr, 500)
							return
						}
						if err := checkToolsNotDropped(name, probe, gotBody, r.URL.Path); err != nil {
							wireErr = err.Error()
							http.Error(w, wireErr, 500)
							return
						}
						payload, err := nativeFixture(name, probe)
						if err != nil {
							wireErr = err.Error()
							http.Error(w, wireErr, 500)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write(payload)
					}))
					defer srv.Close()

					cfg := provider.Config{
						Provider:   name,
						BaseURL:    srv.URL,
						APIKey:     "test-key",
						AccountID:  "acct",
						HTTPClient: srv.Client(),
					}
					if name == provider.OpenAI {
						cfg.BaseURL = srv.URL + "/v1"
					}
					p, err := provider.New(cfg)
					if err != nil {
						t.Fatal(err)
					}
					model := "fixture-model"
					runner := &Runner{Provider: p, Model: model}
					res := runner.Run(context.Background(), probe)
					if wireErr != "" {
						t.Fatalf("request shape: %s", wireErr)
					}
					if !res.Passed {
						t.Fatalf("probe failed on %s wire: %s", name, res.Error)
					}
				})
			}
		})
	}
}

func checkToolsNotDropped(name string, probe *Probe, body map[string]any, path string) error {
	switch name {
	case provider.OpenAI:
		if !strings.HasSuffix(path, "/chat/completions") {
			return errString("openai path " + path)
		}
		tools, _ := body["tools"].([]any)
		if len(tools) != len(probe.Tools) {
			return errString("openai dropped tools")
		}
	case provider.Anthropic:
		if !strings.HasSuffix(path, "/v1/messages") || strings.Contains(path, "chat/completions") {
			return errString("anthropic path " + path)
		}
		tools, _ := body["tools"].([]any)
		if len(tools) != len(probe.Tools) {
			return errString("anthropic dropped tools")
		}
		if _, ok := tools[0].(map[string]any)["input_schema"]; !ok {
			return errString("anthropic tool missing input_schema")
		}
	case provider.Gemini:
		if !strings.Contains(path, ":generateContent") {
			return errString("gemini path " + path)
		}
		tools, _ := body["tools"].([]any)
		if len(tools) != 1 {
			return errString("gemini tools wrapper missing")
		}
		decls, _ := tools[0].(map[string]any)["functionDeclarations"].([]any)
		if len(decls) != len(probe.Tools) {
			return errString("gemini dropped function declarations")
		}
	case provider.Cloudflare:
		if !strings.Contains(path, "/ai/run/") || strings.Contains(path, "chat/completions") {
			return errString("cloudflare path " + path)
		}
		tools, _ := body["tools"].([]any)
		if len(tools) != len(probe.Tools) {
			return errString("cloudflare dropped tools")
		}
		if _, ok := tools[0].(map[string]any)["parameters"]; !ok {
			return errString("cloudflare tool missing parameters")
		}
		if _, wrapped := tools[0].(map[string]any)["function"]; wrapped {
			return errString("cloudflare tool used an OpenAI function wrapper")
		}
	}
	return nil
}

type errString string

func (e errString) Error() string { return string(e) }
