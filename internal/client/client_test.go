package client

import (
	"context"
	"testing"
)

func TestLiveRequiresCreds(t *testing.T) {
	c := New("", "")
	_, err := c.ChatCompletion(context.Background(), ChatRequest{Model: "x"})
	if err == nil {
		t.Fatal("expected error without base-url")
	}
}

func TestMockModeRejectsDirectChat(t *testing.T) {
	c := New("https://example.com/v1", "key")
	c.Mock = true
	_, err := c.ChatCompletion(context.Background(), ChatRequest{Model: "x"})
	if err == nil {
		t.Fatal("expected mock mode error")
	}
}
