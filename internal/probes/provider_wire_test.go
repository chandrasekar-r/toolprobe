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

func nativeFixture(name string, probe *Probe) ([]byte, error) {
	text, calls := mockOutcome(probe)
	switch name {
	case provider.OpenAI:
		msg := map[string]any{"role": "assistant", "content": text}
		if len(calls) > 0 {
			var toolCalls []any
			for i, c := range calls {
				raw, _ := json.Marshal(c.Args)
				toolCalls = append(toolCalls, map[string]any{
					"id":   "call_" + c.Name,
					"type": "function",
					"function": map[string]any{
						"name":      c.Name,
						"arguments": string(raw),
					},
				})
				_ = i
			}
			msg["tool_calls"] = toolCalls
		}
		return json.Marshal(map[string]any{
			"id": "chatcmpl",
			"choices": []any{map[string]any{
				"index":         0,
				"message":       msg,
				"finish_reason": "stop",
			}},
		})
	case provider.Anthropic:
		var content []any
		if text != "" {
			content = append(content, map[string]any{"type": "text", "text": text})
		}
		for i, c := range calls {
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    "toolu_" + string(rune('a'+i)),
				"name":  c.Name,
				"input": c.Args,
			})
		}
		if len(content) == 0 {
			content = []any{map[string]any{"type": "text", "text": ""}}
		}
		stop := "end_turn"
		if len(calls) > 0 {
			stop = "tool_use"
		}
		return json.Marshal(map[string]any{
			"id": "msg", "type": "message", "role": "assistant",
			"stop_reason": stop, "content": content,
		})
	case provider.Gemini:
		var parts []any
		if text != "" {
			parts = append(parts, map[string]any{"text": text})
		}
		for _, c := range calls {
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{"name": c.Name, "args": c.Args},
			})
		}
		if len(parts) == 0 {
			parts = []any{map[string]any{"text": ""}}
		}
		return json.Marshal(map[string]any{
			"candidates": []any{map[string]any{
				"content":      map[string]any{"role": "model", "parts": parts},
				"finishReason": "STOP",
			}},
		})
	case provider.Cloudflare:
		var toolCalls []any
		for _, c := range calls {
			toolCalls = append(toolCalls, map[string]any{"name": c.Name, "arguments": c.Args})
		}
		if toolCalls == nil {
			toolCalls = []any{}
		}
		return json.Marshal(map[string]any{
			"success": true,
			"errors":  []any{},
			"result": map[string]any{
				"response":   text,
				"tool_calls": toolCalls,
			},
		})
	default:
		return nil, errString("unknown provider")
	}
}

type mockCall struct {
	Name string
	Args map[string]any
}

func mockOutcome(p *Probe) (string, []mockCall) {
	spec := p.Mock
	if spec == nil {
		return "", nil
	}
	if spec.Mode == "text" {
		content := spec.Content
		if content == "" {
			content = "No tool needed."
		}
		return content, nil
	}
	if len(spec.ToolCalls) > 0 {
		out := make([]mockCall, 0, len(spec.ToolCalls))
		for _, c := range spec.ToolCalls {
			args := c.Args
			if args == nil {
				args = map[string]any{}
			}
			out = append(out, mockCall{Name: c.Name, Args: args})
		}
		return "", out
	}
	args := spec.Args
	if args == nil {
		args = map[string]any{}
	}
	return "", []mockCall{{Name: spec.ToolName, Args: args}}
}

type errString string

func (e errString) Error() string { return string(e) }
