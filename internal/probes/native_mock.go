package probes

import (
	"fmt"
	"net/http"

	"github.com/chandrasekar-r/toolprobe/internal/provider"
)

// NewNativeMockRunner builds a runner whose Anthropic, Gemini, or Workers AI
// client constructs a real request and parses a probe.mock fixture.
// The HTTP transport does not dial. OpenAI mock stays on the in-process synthesizer.
func NewNativeMockRunner(providerName, model string) (*Runner, error) {
	name, err := provider.Canonical(providerName)
	if err != nil {
		return nil, err
	}
	if name == provider.OpenAI {
		return nil, fmt.Errorf("openai --mock uses the in-process synthesizer, not the native fixture client")
	}
	if model == "" {
		model = provider.MockModel
	}
	off := provider.NewOfflineTransport(name)
	account := ""
	if name == provider.Cloudflare {
		account = provider.MockAccountID
	}
	p, err := provider.New(provider.Config{
		Provider:  name,
		BaseURL:   provider.DefaultBaseURL(name),
		APIKey:    provider.MockAPIKey,
		AccountID: account,
		HTTPClient: &http.Client{
			Transport: off,
		},
	})
	if err != nil {
		return nil, err
	}
	return &Runner{
		Provider: p,
		Model:    model,
		Offline:  off,
	}, nil
}
