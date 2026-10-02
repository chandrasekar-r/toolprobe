package probes

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chandrasekar-r/toolprobe/internal/client"
)

// SynthesizeMock builds a ChatResponse from the probe's MockSpec.
// Used when Client.Mock is true so each probe has deterministic expected behavior.
func SynthesizeMock(p *Probe) (*client.ChatResponse, error) {
	spec := p.Mock
	if spec == nil {
		// Fallback: derive a passing single-tool response from expect.
		spec = &MockSpec{Mode: "tool_calls"}
		if p.Expect.NoTool {
			spec.Mode = "text"
			spec.Content = "I cannot help with that using the available tools."
		} else if len(p.Expect.Calls) > 0 {
			for _, c := range p.Expect.Calls {
				spec.ToolCalls = append(spec.ToolCalls, MockToolCall{Name: c.ToolName, Args: c.Args})
			}
		} else {
			spec.ToolName = p.Expect.ToolName
			spec.Args = p.Expect.Args
		}
	}

	if spec.LatencyMs > 0 {
		time.Sleep(time.Duration(spec.LatencyMs) * time.Millisecond)
	}

	mode := spec.Mode
	if mode == "" {
		mode = "tool_calls"
	}

	switch mode {
	case "text":
		content := spec.Content
		if content == "" {
			content = "No tool needed."
		}
		return &client.ChatResponse{
			ID: "mock-text",
			Choices: []client.Choice{{
				Index:        0,
				Message:      client.Message{Role: "assistant", Content: content},
				FinishReason: "stop",
			}},
		}, nil

	case "empty_args":
		name := firstMockName(spec, p)
		return singleCallResponse(name, "{}"), nil

	case "malformed_args":
		name := firstMockName(spec, p)
		return singleCallResponse(name, "{not-json"), nil

	case "wrong_tool":
		name := "totally_wrong_tool"
		if spec.ToolName != "" {
			name = spec.ToolName
		}
		args := "{}"
		if len(spec.Args) > 0 {
			b, _ := json.Marshal(spec.Args)
			args = string(b)
		}
		return singleCallResponse(name, args), nil

	case "tool_calls":
		calls, err := expandMockCalls(spec)
		if err != nil {
			return nil, err
		}
		if len(calls) == 0 {
			return nil, fmt.Errorf("mock mode tool_calls but no tool calls defined for probe %q", p.Name)
		}
		tc := make([]client.ToolCall, 0, len(calls))
		for i, c := range calls {
			tc = append(tc, client.ToolCall{
				ID:   fmt.Sprintf("call_mock_%d", i),
				Type: "function",
				Function: client.FunctionCall{
					Name:      c.Name,
					Arguments: c.Arguments,
				},
			})
		}
		return &client.ChatResponse{
			ID: "mock-tools",
			Choices: []client.Choice{{
				Index: 0,
				Message: client.Message{
					Role:      "assistant",
					ToolCalls: tc,
				},
				FinishReason: "tool_calls",
			}},
		}, nil

	default:
		return nil, fmt.Errorf("unknown mock mode %q for probe %q", mode, p.Name)
	}
}

func firstMockName(spec *MockSpec, p *Probe) string {
	if spec.ToolName != "" {
		return spec.ToolName
	}
	if len(spec.ToolCalls) > 0 && spec.ToolCalls[0].Name != "" {
		return spec.ToolCalls[0].Name
	}
	if p.Expect.ToolName != "" {
		return p.Expect.ToolName
	}
	return "unknown"
}

func singleCallResponse(name, args string) *client.ChatResponse {
	return &client.ChatResponse{
		ID: "mock-single",
		Choices: []client.Choice{{
			Index: 0,
			Message: client.Message{
				Role: "assistant",
				ToolCalls: []client.ToolCall{{
					ID:   "call_mock_0",
					Type: "function",
					Function: client.FunctionCall{
						Name:      name,
						Arguments: args,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	}
}

type resolvedCall struct {
	Name      string
	Arguments string
}

func expandMockCalls(spec *MockSpec) ([]resolvedCall, error) {
	var raw []MockToolCall
	if len(spec.ToolCalls) > 0 {
		raw = spec.ToolCalls
	} else if spec.ToolName != "" {
		raw = []MockToolCall{{Name: spec.ToolName, Args: spec.Args, Arguments: ""}}
	}
	out := make([]resolvedCall, 0, len(raw))
	for _, c := range raw {
		args := c.Arguments
		if args == "" {
			if c.Args == nil {
				args = "{}"
			} else {
				b, err := json.Marshal(c.Args)
				if err != nil {
					return nil, err
				}
				args = string(b)
			}
		}
		out = append(out, resolvedCall{Name: c.Name, Arguments: args})
	}
	return out, nil
}
