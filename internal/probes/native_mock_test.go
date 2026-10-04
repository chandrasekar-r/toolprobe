package probes

import (
	"context"
	"strings"
	"testing"

	"github.com/chandrasekar-r/toolprobe/internal/provider"
)

func TestNativeMockRunnerUsesProviderClient(t *testing.T) {
	ps, err := LoadDir("../../probes/default")
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 17 {
		t.Fatalf("expected 17 probes, got %d", len(ps))
	}
	cases := []struct {
		name    string
		urlPart string
		bodyHas string
	}{
		{provider.Anthropic, "/v1/messages", "input_schema"},
		{provider.Gemini, ":generateContent", "functionDeclarations"},
		{provider.Cloudflare, "/ai/run/", `"parameters"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := NewNativeMockRunner(tc.name, "")
			if err != nil {
				t.Fatal(err)
			}
			if runner.Model != provider.MockModel {
				t.Fatalf("model %q", runner.Model)
			}
			for _, probe := range ps {
				res := runner.Run(context.Background(), probe)
				if !res.Passed {
					t.Fatalf("%s: %s", probe.Name, res.Error)
				}
			}
			if got := runner.Offline.Calls(); got != len(ps) {
				t.Fatalf("client built %d requests, want %d", got, len(ps))
			}
			method, url, header, body := runner.Offline.LastRequest()
			if method != "POST" {
				t.Fatalf("method %s", method)
			}
			if !strings.Contains(url, tc.urlPart) || strings.Contains(url, "chat/completions") {
				t.Fatalf("url %s", url)
			}
			if !strings.Contains(string(body), tc.bodyHas) {
				t.Fatalf("request body missing %s: %s", tc.bodyHas, body)
			}
			switch tc.name {
			case provider.Anthropic:
				if header.Get("x-api-key") == "" || header.Get("anthropic-version") == "" {
					t.Fatal("anthropic headers were not built")
				}
			case provider.Gemini:
				if header.Get("x-goog-api-key") == "" || strings.Contains(url, "key=") {
					t.Fatalf("gemini auth url=%s", url)
				}
			case provider.Cloudflare:
				if !strings.HasPrefix(header.Get("Authorization"), "Bearer ") {
					t.Fatal("workers ai authorization was not built")
				}
			}
		})
	}
}

func TestNativeMockRejectsOpenAI(t *testing.T) {
	if _, err := NewNativeMockRunner(provider.OpenAI, ""); err == nil {
		t.Fatal("openai mock must stay on the synthesizer")
	}
}
