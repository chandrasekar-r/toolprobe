package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chandrasekar-r/toolprobe/internal/client"
)

type openAIProvider struct {
	client *client.Client
}

func (o *openAIProvider) Name() string { return OpenAI }

func (o *openAIProvider) Complete(ctx context.Context, turn Turn) (*Result, error) {
	if err := validateTools(turn.Tools); err != nil {
		return nil, err
	}
	tools := make([]client.Tool, 0, len(turn.Tools))
	for _, tool := range turn.Tools {
		params, err := json.Marshal(parametersOrEmpty(tool.Parameters))
		if err != nil {
			return nil, fmt.Errorf("marshal parameters for %s: %w", tool.Name, err)
		}
		tools = append(tools, client.Tool{
			Type: "function",
			Function: client.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		})
	}
	msgs := make([]client.Message, 0, 2)
	if turn.System != "" {
		msgs = append(msgs, client.Message{Role: "system", Content: turn.System})
	}
	msgs = append(msgs, client.Message{Role: "user", Content: turn.User})
	resp, err := o.client.ChatCompletion(ctx, client.ChatRequest{
		Model:    turn.Model,
		Messages: msgs,
		Tools:    tools,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return nil, errf("no choices in response")
	}
	msg := resp.Choices[0].Message
	out := &Result{Text: msg.Content, ToolCalls: make([]Call, 0, len(msg.ToolCalls))}
	for _, call := range msg.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, Call{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}
	return out, nil
}
