package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// geminiProvider calls POST /v1beta/models/{model}:generateContent.
// Function declarations use the public generateContent tools shape
// (functionDeclarations + parameters). The model id is whatever the caller
// passes; this package does not invent one.
type geminiProvider struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func (g *geminiProvider) Name() string { return Gemini }

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	Tools             []geminiTool    `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiDecl `json:"functionDeclarations"`
}

type geminiDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string `json:"text,omitempty"`
				FunctionCall *struct {
					Name string          `json:"name"`
					Args json.RawMessage `json:"args"`
					ID   string          `json:"id,omitempty"`
				} `json:"functionCall,omitempty"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason,omitempty"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback,omitempty"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (g *geminiProvider) Complete(ctx context.Context, turn Turn) (*Result, error) {
	if g.apiKey == "" {
		return nil, errf("api-key is required for gemini (or use --mock)")
	}
	endpoint, err := geminiGenerateURL(g.baseURL, turn.Model)
	if err != nil {
		return nil, err
	}
	if err := validateTools(turn.Tools); err != nil {
		return nil, err
	}
	req := geminiRequest{
		Contents: []geminiContent{{
			Role:  "user",
			Parts: []geminiPart{{Text: turn.User}},
		}},
	}
	if turn.System != "" {
		req.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: turn.System}}}
	}
	if len(turn.Tools) > 0 {
		decls := make([]geminiDecl, 0, len(turn.Tools))
		for _, tool := range turn.Tools {
			decls = append(decls, geminiDecl{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  parametersOrEmpty(tool.Parameters),
			})
		}
		req.Tools = []geminiTool{{FunctionDeclarations: decls}}
	}
	status, body, err := postJSON(ctx, g.http, endpoint, map[string]string{
		"x-goog-api-key": g.apiKey,
	}, req)
	if err != nil {
		return nil, err
	}
	var parsed geminiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		if status < 200 || status >= 300 {
			return nil, statusErr("gemini", status, body)
		}
		return nil, errf("decode gemini response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, errf("gemini: %s", parsed.Error.Message)
	}
	if status < 200 || status >= 300 {
		return nil, statusErr("gemini", status, body)
	}
	if parsed.PromptFeedback != nil && parsed.PromptFeedback.BlockReason != "" && len(parsed.Candidates) == 0 {
		return nil, errf("gemini blocked the prompt: %s", parsed.PromptFeedback.BlockReason)
	}
	if len(parsed.Candidates) == 0 {
		return nil, errf("gemini returned no candidates")
	}
	out := &Result{}
	var texts []string
	for i, part := range parsed.Candidates[0].Content.Parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
		if part.FunctionCall == nil {
			continue
		}
		args, err := normalizeArgs(part.FunctionCall.Args)
		if err != nil {
			return nil, fmt.Errorf("gemini functionCall %q: %w", part.FunctionCall.Name, err)
		}
		id := part.FunctionCall.ID
		if id == "" {
			id = fmt.Sprintf("fc_%d", i)
		}
		out.ToolCalls = append(out.ToolCalls, Call{
			ID:        id,
			Name:      part.FunctionCall.Name,
			Arguments: args,
		})
	}
	out.Text = strings.Join(texts, "")
	return out, nil
}

func geminiGenerateURL(base, model string) (string, error) {
	model = strings.TrimSpace(model)
	model = strings.TrimPrefix(model, "models/")
	if model == "" {
		return "", errf("model is required for gemini (pass --model; toolprobe does not default a Gemini model id)")
	}
	if strings.ContainsAny(model, "/?#") {
		return "", errf("invalid gemini model %q", model)
	}
	base = strings.TrimRight(base, "/")
	base = strings.TrimSuffix(base, "/v1beta")
	base = strings.TrimSuffix(base, "/v1")
	return base + "/v1beta/models/" + url.PathEscape(model) + ":generateContent", nil
}
