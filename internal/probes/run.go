package probes

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/chandrasekar-r/toolprobe/internal/client"
)

// Runner executes probes against a chat client.
type Runner struct {
	Client *client.Client
	Model  string
}

// Run executes a single probe and returns the result.
func (r *Runner) Run(ctx context.Context, p *Probe) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		Name:     p.Name,
		Expected: p.Expect,
	}

	tools, err := toClientTools(p.Tools)
	if err != nil {
		result.Error = err.Error()
		result.Latency = time.Since(start)
		result.LatencyMs = float64(result.Latency.Microseconds()) / 1000.0
		return result
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
	result.Latency = time.Since(start)
	result.LatencyMs = float64(result.Latency.Microseconds()) / 1000.0
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if len(resp.Choices) == 0 {
		result.Error = "no choices in response"
		return result
	}

	calls := resp.Choices[0].Message.ToolCalls
	if len(calls) == 0 {
		result.Error = "model returned no tool calls"
		return result
	}

	call := calls[0]
	result.ToolName = call.Function.Name
	result.ToolArgs = call.Function.Arguments

	if call.Function.Name != p.Expect.ToolName {
		result.Error = fmt.Sprintf("tool name: got %q want %q", call.Function.Name, p.Expect.ToolName)
		return result
	}

	var gotArgs map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &gotArgs); err != nil {
		result.Error = fmt.Sprintf("tool args not valid JSON: %v", err)
		return result
	}

	if !argsMatch(gotArgs, p.Expect.Args) {
		want, _ := json.Marshal(p.Expect.Args)
		result.Error = fmt.Sprintf("tool args mismatch: got %s want %s", call.Function.Arguments, string(want))
		return result
	}

	result.Passed = true
	return result
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

func valuesEqual(a, b interface{}) bool {
	// Normalize JSON number types: yaml may give int, json may give float64.
	na, okA := toFloat(a)
	nb, okB := toFloat(b)
	if okA && okB {
		return na == nb
	}
	sa, okA := a.(string)
	sb, okB := b.(string)
	if okA && okB {
		return sa == sb
	}
	ba, okA := a.(bool)
	bb, okB := b.(bool)
	if okA && okB {
		return ba == bb
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
