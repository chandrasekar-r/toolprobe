package client

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMockChatCompletion_Weather(t *testing.T) {
	c := New("", "")
	c.Mock = true

	req := ChatRequest{
		Model: "mock",
		Messages: []Message{
			{Role: "user", Content: "What is the weather in Berlin in celsius?"},
		},
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_weather",
				Description: "Get weather",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}},
	}

	resp, err := c.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("no choices")
	}
	calls := resp.Choices[0].Message.ToolCalls
	if len(calls) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "get_weather" {
		t.Fatalf("name = %q", calls[0].Function.Name)
	}
	var args map[string]string
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("args json: %v", err)
	}
	if args["city"] != "Berlin" || args["units"] != "celsius" {
		t.Fatalf("args = %#v", args)
	}
}

func TestLiveRequiresCreds(t *testing.T) {
	c := New("", "")
	_, err := c.ChatCompletion(context.Background(), ChatRequest{Model: "x"})
	if err == nil {
		t.Fatal("expected error without base-url")
	}
}
