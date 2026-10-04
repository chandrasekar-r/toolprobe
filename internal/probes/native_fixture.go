package probes

import (
	"encoding/json"
	"fmt"

	"github.com/chandrasekar-r/toolprobe/internal/provider"
)

// nativeFixture encodes probe.mock as the provider's own response JSON.
// The provider client parses these bytes with the same decoder it uses for a live response.
func nativeFixture(name string, probe *Probe) ([]byte, error) {
	text, calls := mockOutcome(probe)
	switch name {
	case provider.OpenAI:
		msg := map[string]any{"role": "assistant", "content": text}
		if len(calls) > 0 {
			var toolCalls []any
			for _, c := range calls {
				raw, err := json.Marshal(c.Args)
				if err != nil {
					return nil, err
				}
				toolCalls = append(toolCalls, map[string]any{
					"id":   "call_" + c.Name,
					"type": "function",
					"function": map[string]any{
						"name":      c.Name,
						"arguments": string(raw),
					},
				})
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
				"id":    fmt.Sprintf("toolu_%d", i),
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
			"id": "msg_mock", "type": "message", "role": "assistant",
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
		return nil, fmt.Errorf("no native fixture for provider %q", name)
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
