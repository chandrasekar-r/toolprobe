package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// cloudflareProvider calls POST /accounts/{account_id}/ai/run/{model}.
// Tools use the traditional Workers AI shape (name, description, parameters),
// not an OpenAI chat/completions body. Responses are the REST envelope
// {success, errors, result:{response, tool_calls}} or the binding object
// {response, tool_calls}. If the model returns a non-chat result, Complete
// fails instead of dropping tools.
type cloudflareProvider struct {
	baseURL   string
	apiKey    string
	accountID string
	http      *http.Client
}

func (c *cloudflareProvider) Name() string { return Cloudflare }

type cfMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cfTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type cfRequest struct {
	Messages []cfMessage `json:"messages"`
	Tools    []cfTool    `json:"tools,omitempty"`
}

func (c *cloudflareProvider) Complete(ctx context.Context, turn Turn) (*Result, error) {
	if c.apiKey == "" {
		return nil, errf("api-key is required for cloudflare (or use --mock)")
	}
	endpoint, err := cloudflareRunURL(c.baseURL, c.accountID, turn.Model)
	if err != nil {
		return nil, err
	}
	if err := validateTools(turn.Tools); err != nil {
		return nil, err
	}
	req := cfRequest{}
	if turn.System != "" {
		req.Messages = append(req.Messages, cfMessage{Role: "system", Content: turn.System})
	}
	req.Messages = append(req.Messages, cfMessage{Role: "user", Content: turn.User})
	if len(turn.Tools) > 0 {
		req.Tools = make([]cfTool, 0, len(turn.Tools))
		for _, tool := range turn.Tools {
			req.Tools = append(req.Tools, cfTool{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  parametersOrEmpty(tool.Parameters),
			})
		}
	}
	status, body, err := postJSON(ctx, c.http, endpoint, map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	}, req)
	if err != nil {
		return nil, err
	}
	out, err := parseWorkersAI(body, len(turn.Tools) > 0)
	if err != nil {
		if status < 200 || status >= 300 {
			return nil, statusErr("cloudflare", status, body)
		}
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, statusErr("cloudflare", status, body)
	}
	return out, nil
}

func cloudflareRunURL(base, account, model string) (string, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return "", errf("cloudflare account id is required (--account-id, CLOUDFLARE_ACCOUNT_ID, or TOOLPROBE_CLOUDFLARE_ACCOUNT_ID)")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", errf("model is required for cloudflare (pass --model; use a catalog model marked Function calling)")
	}
	if strings.Contains(model, "..") || strings.ContainsAny(model, "?# ") {
		return "", errf("invalid workers ai model %q", model)
	}
	base = strings.TrimRight(base, "/")
	return base + "/accounts/" + url.PathEscape(account) + "/ai/run/" + model, nil
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func parseWorkersAI(body []byte, toolsSent bool) (*Result, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, errf("decode workers ai response: %w", err)
	}
	if raw, ok := top["errors"]; ok {
		var errs []cfError
		if err := json.Unmarshal(raw, &errs); err == nil && len(errs) > 0 {
			return nil, workersAIErrors(errs)
		}
	}
	if raw, ok := top["success"]; ok {
		var okSuccess bool
		if err := json.Unmarshal(raw, &okSuccess); err == nil && !okSuccess {
			return nil, errf("workers ai: request failed (success=false)")
		}
	}
	payload := body
	if raw, ok := top["result"]; ok {
		payload = raw
	} else if _, hasResponse := top["response"]; !hasResponse {
		if _, hasCalls := top["tool_calls"]; !hasCalls {
			return nil, errf("workers ai: response has no result")
		}
	}
	payload = bytesTrim(payload)
	if len(payload) == 0 || string(payload) == "null" {
		return nil, errf("workers ai: empty result")
	}
	switch payload[0] {
	case '"':
		var text string
		_ = json.Unmarshal(payload, &text)
		return nil, errf("workers ai model returned a plain string and no tool_calls (%s); it does not support function calling. tools were sent and were not dropped", truncate(text, 180))
	case '[':
		return nil, errf("workers ai model returned a non-chat result (JSON array); it does not support function calling. tools were sent and were not dropped")
	case '{':
		// chat object
	default:
		return nil, errf("workers ai model returned an unexpected result; it does not support function calling. tools were sent and were not dropped")
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, errf("decode workers ai result: %w", err)
	}
	out := &Result{}
	if raw, ok := obj["response"]; ok {
		_ = json.Unmarshal(raw, &out.Text)
	}
	rawCalls, hasCalls := obj["tool_calls"]
	if !hasCalls || string(bytesTrim(rawCalls)) == "null" {
		if toolsSent {
			out.Warning = "workers ai response has no tool_calls field; if this model is not marked Function calling in the catalog, it ignored the tools that were sent"
		}
		return out, nil
	}
	var calls []struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(rawCalls, &calls); err != nil {
		return nil, errf("workers ai tool_calls: %w", err)
	}
	for i, call := range calls {
		args, err := normalizeArgs(call.Arguments)
		if err != nil {
			return nil, fmt.Errorf("workers ai tool %q: %w", call.Name, err)
		}
		out.ToolCalls = append(out.ToolCalls, Call{
			ID:        fmt.Sprintf("cf_%d", i),
			Name:      call.Name,
			Arguments: args,
		})
	}
	return out, nil
}

func workersAIErrors(errs []cfError) error {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		if e.Message == "" {
			continue
		}
		if e.Code != 0 {
			parts = append(parts, fmt.Sprintf("%d %s", e.Code, e.Message))
		} else {
			parts = append(parts, e.Message)
		}
	}
	if len(parts) == 0 {
		return errf("workers ai: request failed")
	}
	msg := strings.Join(parts, "; ")
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "tool") || strings.Contains(lower, "function") || strings.Contains(lower, "not support") {
		return errf("workers ai model does not support tools: %s. tools were sent and were not dropped", msg)
	}
	return errf("workers ai: %s", msg)
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
