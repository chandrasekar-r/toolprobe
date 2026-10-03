// Package provider talks to native tool-calling APIs.
//
// OpenAI-compatible chat/completions stays a first-class provider. Anthropic
// Messages, Gemini generateContent, and Cloudflare Workers AI /ai/run each
// send and parse that provider's own JSON. They are not rewritten as
// chat/completions.
package provider

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/chandrasekar-r/toolprobe/internal/client"
)

const (
	// OpenAI is the OpenAI-compatible chat/completions provider.
	OpenAI = "openai"
	// Anthropic is the Anthropic Messages API (tool use).
	Anthropic = "anthropic"
	// Gemini is the Google Gemini generateContent API (function calling).
	Gemini = "gemini"
	// Cloudflare is the Workers AI REST /ai/run API (traditional function calling).
	Cloudflare = "cloudflare"

	// AnthropicVersion is the documented Messages API version header.
	AnthropicVersion = "2023-06-01"
	// AnthropicMaxTokens is sent because the Messages API requires max_tokens.
	// Tool-call probes only need a short JSON tool_use block.
	AnthropicMaxTokens = 1024
)

// Tool is one function offered to the model.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Turn is one probe turn in provider-neutral form.
type Turn struct {
	Model  string
	System string
	User   string
	Tools  []Tool
}

// Call is one tool invocation. Arguments is a JSON object string.
type Call struct {
	ID        string
	Name      string
	Arguments string
}

// Result is a normalized model turn.
type Result struct {
	Text      string
	ToolCalls []Call
	// Warning is extra context for a failed probe (for example, Workers AI
	// returned text and omitted tool_calls). It is not itself a failure.
	Warning string
}

// Provider sends one turn to a native API.
type Provider interface {
	Name() string
	Complete(ctx context.Context, turn Turn) (*Result, error)
}

// Config selects and authenticates a provider. Keys come from the caller;
// this package never embeds a default key.
type Config struct {
	Provider   string
	BaseURL    string
	APIKey     string
	AccountID  string
	Mock       bool
	HTTPClient *http.Client
}

// Canonical maps aliases onto openai, anthropic, gemini, or cloudflare.
func Canonical(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "openai", "openai-compatible", "chat-completions":
		return OpenAI, nil
	case "anthropic", "claude":
		return Anthropic, nil
	case "gemini", "google", "google-gemini":
		return Gemini, nil
	case "cloudflare", "workers-ai", "workers_ai", "cloudflare-workers-ai":
		return Cloudflare, nil
	default:
		return "", errf("unknown provider %q (want openai, anthropic, gemini, or cloudflare)", name)
	}
}

// DefaultBaseURL is the public API origin for providers that have one.
// OpenAI has no default: callers pass --base-url, matching the original CLI.
// Gemini's value is the generateContent host only. It is not a model id.
func DefaultBaseURL(name string) string {
	switch name {
	case Anthropic:
		return "https://api.anthropic.com"
	case Gemini:
		return "https://generativelanguage.googleapis.com"
	case Cloudflare:
		return "https://api.cloudflare.com/client/v4"
	default:
		return ""
	}
}

// New builds a live provider. Mock configurations still return a named
// provider so reports can record the selection; Complete refuses mock clients
// that wrap the OpenAI client, and the runner synthesizes --mock itself.
func New(cfg Config) (Provider, error) {
	name, err := Canonical(cfg.Provider)
	if err != nil {
		return nil, err
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL(name)
	}

	switch name {
	case OpenAI:
		if !cfg.Mock {
			if base == "" {
				return nil, errf("base-url is required (or use --mock)")
			}
			if cfg.APIKey == "" {
				return nil, errf("api-key is required (or use --mock)")
			}
		}
		c := client.New(base, cfg.APIKey)
		c.HTTPClient = hc
		c.Mock = cfg.Mock
		return &openAIProvider{client: c}, nil
	case Anthropic:
		if !cfg.Mock && cfg.APIKey == "" {
			return nil, errf("api-key is required for anthropic (ANTHROPIC_API_KEY, TOOLPROBE_ANTHROPIC_API_KEY, or TOOLPROBE_API_KEY)")
		}
		return &anthropicProvider{baseURL: base, apiKey: cfg.APIKey, http: hc}, nil
	case Gemini:
		if !cfg.Mock && cfg.APIKey == "" {
			return nil, errf("api-key is required for gemini (GEMINI_API_KEY, GOOGLE_API_KEY, TOOLPROBE_GEMINI_API_KEY, or TOOLPROBE_API_KEY)")
		}
		return &geminiProvider{baseURL: base, apiKey: cfg.APIKey, http: hc}, nil
	case Cloudflare:
		if !cfg.Mock && cfg.APIKey == "" {
			return nil, errf("api-key is required for cloudflare (CLOUDFLARE_API_TOKEN, TOOLPROBE_CLOUDFLARE_API_TOKEN, or TOOLPROBE_API_KEY)")
		}
		if !cfg.Mock && strings.TrimSpace(cfg.AccountID) == "" {
			return nil, errf("cloudflare account id is required (--account-id, CLOUDFLARE_ACCOUNT_ID, or TOOLPROBE_CLOUDFLARE_ACCOUNT_ID)")
		}
		return &cloudflareProvider{baseURL: base, apiKey: cfg.APIKey, accountID: cfg.AccountID, http: hc}, nil
	default:
		return nil, errf("unknown provider %q", name)
	}
}

func parametersOrEmpty(p map[string]any) map[string]any {
	if len(p) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return p
}

func validateTools(tools []Tool) error {
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return errf("tool name is required")
		}
	}
	return nil
}
