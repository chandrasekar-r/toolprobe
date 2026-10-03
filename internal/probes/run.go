package probes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chandrasekar-r/toolprobe/internal/client"
	"github.com/chandrasekar-r/toolprobe/internal/provider"
)

// Runner executes probes against a chat client or a native provider.
type Runner struct {
	Client   *client.Client
	Provider provider.Provider
	Model    string
}

// Run executes a single probe and returns the result.
func (r *Runner) Run(ctx context.Context, p *Probe) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		Name:     p.Name,
		Expected: p.Expect,
	}

	calls, warning, err := r.invoke(ctx, p)
	result.Latency = time.Since(start)
	result.LatencyMs = float64(result.Latency.Microseconds()) / 1000.0
	finish := func() ProbeResult {
		if !result.Passed && warning != "" {
			if result.Error != "" {
				result.Error += "; " + warning
			} else {
				result.Error = warning
			}
		}
		return result
	}
	if err != nil {
		result.Error = err.Error()
		return finish()
	}
	if len(calls) > 0 {
		result.ToolName = calls[0].Function.Name
		result.ToolArgs = calls[0].Function.Arguments
	}

	if p.Expect.MaxLatencyMs > 0 && result.LatencyMs > p.Expect.MaxLatencyMs {
		result.Error = fmt.Sprintf("latency %.1fms exceeds max_latency_ms %.1f", result.LatencyMs, p.Expect.MaxLatencyMs)
		return finish()
	}

	if p.Expect.NoTool {
		if len(calls) > 0 {
			result.Error = fmt.Sprintf("expected no tool calls, got %q", calls[0].Function.Name)
			return finish()
		}
		result.Passed = true
		return finish()
	}

	if len(p.Expect.Calls) > 0 {
		if err := matchMultiCalls(calls, p.Expect.Calls); err != nil {
			result.Error = err.Error()
			return finish()
		}
		result.Passed = true
		return finish()
	}

	// Single tool call expectation.
	if len(calls) == 0 {
		result.Error = "model returned no tool calls"
		return finish()
	}
	call := calls[0]
	if call.Function.Name != p.Expect.ToolName {
		result.Error = fmt.Sprintf("tool name: got %q want %q", call.Function.Name, p.Expect.ToolName)
		return finish()
	}
	var gotArgs map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &gotArgs); err != nil {
		result.Error = fmt.Sprintf("tool args not valid JSON: %v", err)
		return finish()
	}
	if !argsMatch(gotArgs, p.Expect.Args) {
		want, _ := json.Marshal(p.Expect.Args)
		result.Error = fmt.Sprintf("tool args mismatch: got %s want %s", call.Function.Arguments, string(want))
		return finish()
	}
	result.Passed = true
	return finish()
}

func (r *Runner) invoke(ctx context.Context, p *Probe) ([]client.ToolCall, string, error) {
	if r.Client != nil && r.Client.Mock {
		resp, err := SynthesizeMock(p)
		if err != nil {
			return nil, "", err
		}
		if resp == nil || len(resp.Choices) == 0 {
			return nil, "", fmt.Errorf("no choices in response")
		}
		return resp.Choices[0].Message.ToolCalls, "", nil
	}
	if r.Provider != nil {
		res, err := r.Provider.Complete(ctx, provider.Turn{
			Model:  r.Model,
			System: p.System,
			User:   p.User,
			Tools:  toProviderTools(p.Tools),
		})
		if err != nil {
			return nil, "", err
		}
		if res == nil {
			return nil, "", fmt.Errorf("no response")
		}
		return fromProviderCalls(res.ToolCalls), res.Warning, nil
	}
	if r.Client == nil {
		return nil, "", fmt.Errorf("no provider configured")
	}
	tools, err := toClientTools(p.Tools)
	if err != nil {
		return nil, "", err
	}
	msgs := []client.Message{}
	if p.System != "" {
		msgs = append(msgs, client.Message{Role: "system", Content: p.System})
	}
	msgs = append(msgs, client.Message{Role: "user", Content: p.User})
	resp, err := r.Client.ChatCompletion(ctx, client.ChatRequest{
		Model:    r.Model,
		Messages: msgs,
		Tools:    tools,
	})
	if err != nil {
		return nil, "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return nil, "", fmt.Errorf("no choices in response")
	}
	return resp.Choices[0].Message.ToolCalls, "", nil
}

func toProviderTools(defs []ToolDef) []provider.Tool {
	out := make([]provider.Tool, 0, len(defs))
	for _, d := range defs {
		out = append(out, provider.Tool{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  d.Parameters,
		})
	}
	return out
}

func fromProviderCalls(calls []provider.Call) []client.ToolCall {
	out := make([]client.ToolCall, 0, len(calls))
	for i, c := range calls {
		id := c.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		out = append(out, client.ToolCall{
			ID:   id,
			Type: "function",
			Function: client.FunctionCall{
				Name:      c.Name,
				Arguments: c.Arguments,
			},
		})
	}
	return out
}

func matchMultiCalls(got []client.ToolCall, want []ExpectCall) error {
	if len(got) < len(want) {
		return fmt.Errorf("tool calls: got %d want at least %d", len(got), len(want))
	}
	used := make([]bool, len(got))
	for _, w := range want {
		matched := false
		for i, g := range got {
			if used[i] {
				continue
			}
			if g.Function.Name != w.ToolName {
				continue
			}
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(g.Function.Arguments), &args); err != nil {
				continue
			}
			if argsMatch(args, w.Args) {
				used[i] = true
				matched = true
				break
			}
		}
		if !matched {
			wantJSON, _ := json.Marshal(w.Args)
			return fmt.Errorf("missing expected call %s %s", w.ToolName, string(wantJSON))
		}
	}
	return nil
}

func toClientTools(defs []ToolDef) ([]client.Tool, error) {
	out := make([]client.Tool, 0, len(defs))
	for _, d := range defs {
		params, err := json.Marshal(d.Parameters)
		if err != nil {
			return nil, fmt.Errorf("marshal parameters for %s: %w", d.Name, err)
		}
		out = append(out, client.Tool{
			Type: "function",
			Function: client.ToolFunction{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  params,
			},
		})
	}
	return out, nil
}

// argsMatch checks that every expected key/value is present in got (extra keys OK).
func argsMatch(got, want map[string]interface{}) bool {
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			return false
		}
		if !valuesEqual(gv, wv) {
			return false
		}
	}
	return true
}
