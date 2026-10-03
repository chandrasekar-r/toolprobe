package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// anthropicProvider calls POST /v1/messages.
// The body matches the public Messages API: tools carry input_schema, and
// tool calls are content blocks with type "tool_use".
type anthropicProvider struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func (a *anthropicProvider) Name() string { return Anthropic }

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []anthropicMsg  `json:"messages"`
	Tools     []anthropicTool `json:"tools,omitempty"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content"`
	StopReason string `json:"stop_reason,omitempty"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *anthropicProvider) Complete(ctx context.Context, turn Turn) (*Result, error) {
	if a.apiKey == "" {
		return nil, errf("api-key is required for anthropic (or use --mock)")
	}
	if strings.TrimSpace(turn.Model) == "" {
		return nil, errf("model is required for anthropic")
	}
	if err := validateTools(turn.Tools); err != nil {
		return nil, err
	}
	req := anthropicRequest{
		Model:     turn.Model,
		MaxTokens: AnthropicMaxTokens,
		System:    turn.System,
		Messages:  []anthropicMsg{{Role: "user", Content: turn.User}},
	}
	if len(turn.Tools) > 0 {
		req.Tools = make([]anthropicTool, 0, len(turn.Tools))
		for _, tool := range turn.Tools {
			req.Tools = append(req.Tools, anthropicTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: parametersOrEmpty(tool.Parameters),
			})
		}
	}
	status, body, err := postJSON(ctx, a.http, anthropicMessagesURL(a.baseURL), map[string]string{
		"x-api-key":         a.apiKey,
		"anthropic-version": AnthropicVersion,
	}, req)
	if err != nil {
		return nil, err
	}
	var parsed anthropicResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		if status < 200 || status >= 300 {
			return nil, statusErr("anthropic", status, body)
		}
		return nil, errf("decode anthropic response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, errf("anthropic: %s", parsed.Error.Message)
	}
	if status < 200 || status >= 300 {
		return nil, statusErr("anthropic", status, body)
	}
	out := &Result{}
	var texts []string
	for i, block := range parsed.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				texts = append(texts, block.Text)
			}
		case "tool_use":
			args, err := normalizeArgs(block.Input)
			if err != nil {
				return nil, fmt.Errorf("anthropic tool_use %q: %w", block.Name, err)
			}
			id := block.ID
			if id == "" {
				id = fmt.Sprintf("toolu_%d", i)
			}
			out.ToolCalls = append(out.ToolCalls, Call{ID: id, Name: block.Name, Arguments: args})
		}
	}
	out.Text = strings.Join(texts, "")
	return out, nil
}

func anthropicMessagesURL(base string) string {
	base = strings.TrimRight(base, "/")
	switch {
	case strings.HasSuffix(base, "/v1/messages"):
		return base
	case strings.HasSuffix(base, "/v1"):
		return base + "/messages"
	default:
		return base + "/v1/messages"
	}
}
