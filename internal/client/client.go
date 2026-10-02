// Package client provides an OpenAI-compatible chat completions client with tools.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ToolFunction describes a function tool for the model.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// Tool is an OpenAI-style tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// Message is a chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall is a model-requested tool invocation.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the name and JSON arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatRequest is an OpenAI-compatible chat completions request.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
}

// ChatResponse is an OpenAI-compatible chat completions response.
type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice is one completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage holds token counts when present.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Client talks to an OpenAI-compatible /v1/chat/completions endpoint.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Mock       bool
}

// New creates a client. baseURL should be like https://api.openai.com/v1 (no trailing slash required).
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ChatCompletion sends a chat request and returns the response.
// In Mock mode, returns a canned weather tool call without network I/O.
func (c *Client) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if c.Mock {
		return mockWeatherResponse(req), nil
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("base-url is required (or use --mock)")
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("api-key is required (or use --mock)")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("api status %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	var out ChatResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// mockWeatherResponse returns a successful get_weather tool call for the default probe.
func mockWeatherResponse(req ChatRequest) *ChatResponse {
	city := "Berlin"
	units := "celsius"
	// Best-effort: pull hints from user message if present.
	for _, m := range req.Messages {
		if m.Role != "user" {
			continue
		}
		lower := strings.ToLower(m.Content)
		if strings.Contains(lower, "fahrenheit") {
			units = "fahrenheit"
		}
		if strings.Contains(lower, "berlin") {
			city = "Berlin"
		} else if strings.Contains(lower, "paris") {
			city = "Paris"
		} else if strings.Contains(lower, "tokyo") {
			city = "Tokyo"
		}
	}
	args, _ := json.Marshal(map[string]string{"city": city, "units": units})
	return &ChatResponse{
		ID: "mock-chatcmpl",
		Choices: []Choice{{
			Index: 0,
			Message: Message{
				Role: "assistant",
				ToolCalls: []ToolCall{{
					ID:   "call_mock_weather",
					Type: "function",
					Function: FunctionCall{
						Name:      "get_weather",
						Arguments: string(args),
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	}
}
