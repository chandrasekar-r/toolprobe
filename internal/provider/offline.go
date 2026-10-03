package provider

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

// MockModel is a placeholder model id used only to build native requests in
// --mock. It is not a published model and a mock run does not call a provider.
const MockModel = "toolprobe-mock"

// MockAPIKey is a non-secret placeholder so the client can set auth headers
// on a request that the offline transport never sends.
const MockAPIKey = "mock"

// MockAccountID is a placeholder Cloudflare account id for offline mock URLs.
const MockAccountID = "mock-account"

// OfflineTransport answers provider HTTP calls without dialing.
// RoundTrip reads the request the client built, checks that it is that
// provider's native JSON, and returns the fixture body last passed to Prepare.
type OfflineTransport struct {
	provider string

	mu        sync.Mutex
	status    int
	body      []byte
	toolCount int

	calls    int
	lastReq  *http.Request
	lastBody []byte
}

// NewOfflineTransport returns a transport for provider (anthropic, gemini, or cloudflare).
func NewOfflineTransport(providerName string) *OfflineTransport {
	return &OfflineTransport{provider: providerName}
}

// Prepare sets the fixture response for the next RoundTrip.
// toolCount is how many tools the probe offered; the built request must list them.
func (t *OfflineTransport) Prepare(status int, body []byte, toolCount int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if status == 0 {
		status = http.StatusOK
	}
	t.status = status
	t.body = append([]byte(nil), body...)
	t.toolCount = toolCount
}

// Calls is how many requests the provider client built.
func (t *OfflineTransport) Calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

// LastRequest returns the last request the client built, including its body.
func (t *OfflineTransport) LastRequest() (method, url string, header http.Header, body []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastReq == nil {
		return "", "", nil, nil
	}
	return t.lastReq.Method, t.lastReq.URL.String(), t.lastReq.Header.Clone(), append([]byte(nil), t.lastBody...)
}

// RoundTrip implements http.RoundTripper. It does not dial.
func (t *OfflineTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r == nil || r.URL == nil {
		return nil, errf("offline: provider did not build a request")
	}
	if r.Body == nil {
		return nil, errf("offline: provider built a request with no body")
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, errf("offline: read request: %w", err)
	}
	_ = r.Body.Close()
	if err := t.checkRequest(r, raw); err != nil {
		return nil, err
	}

	t.mu.Lock()
	status := t.status
	fixture := append([]byte(nil), t.body...)
	t.calls++
	t.lastReq = r.Clone(r.Context())
	t.lastBody = append([]byte(nil), raw...)
	t.mu.Unlock()

	if status == 0 || len(fixture) == 0 {
		return nil, errf("offline: no fixture response prepared")
	}
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(fixture)),
		ContentLength: int64(len(fixture)),
		Request:       r,
	}, nil
}

func (t *OfflineTransport) checkRequest(r *http.Request, raw []byte) error {
	if r.Method != http.MethodPost {
		return errf("offline: expected POST, got %s", r.Method)
	}
	if !json.Valid(raw) {
		return errf("offline: request body is not JSON")
	}
	path := r.URL.Path
	if strings.Contains(path, "chat/completions") {
		return errf("offline: %s mock built an OpenAI chat/completions request", t.provider)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return errf("offline: decode request: %w", err)
	}
	switch t.provider {
	case Anthropic:
		if !strings.HasSuffix(path, "/v1/messages") {
			return errf("offline: anthropic path %s", path)
		}
		if r.Header.Get("x-api-key") == "" || r.Header.Get("anthropic-version") == "" {
			return errf("offline: anthropic request is missing x-api-key or anthropic-version")
		}
		tools, _ := body["tools"].([]any)
		if err := expectToolCount(t.toolCount, len(tools)); err != nil {
			return err
		}
		if len(tools) > 0 {
			if _, ok := tools[0].(map[string]any)["input_schema"]; !ok {
				return errf("offline: anthropic tool missing input_schema")
			}
		}
	case Gemini:
		if !strings.Contains(path, ":generateContent") {
			return errf("offline: gemini path %s", path)
		}
		if r.URL.RawQuery != "" {
			return errf("offline: gemini key must be a header, not a query string")
		}
		if r.Header.Get("x-goog-api-key") == "" {
			return errf("offline: gemini request is missing x-goog-api-key")
		}
		tools, _ := body["tools"].([]any)
		decls := 0
		if len(tools) == 1 {
			tool0, ok := tools[0].(map[string]any)
			if !ok {
				return errf("offline: gemini tools[0] is not an object")
			}
			decls, _ = countSlice(tool0["functionDeclarations"])
		}
		if t.toolCount > 0 && len(tools) != 1 {
			return errf("offline: gemini request dropped functionDeclarations")
		}
		if err := expectToolCount(t.toolCount, decls); err != nil {
			return err
		}
	case Cloudflare:
		if !strings.Contains(path, "/ai/run/") {
			return errf("offline: workers ai path %s", path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			return errf("offline: workers ai request is missing Authorization")
		}
		tools, _ := body["tools"].([]any)
		if err := expectToolCount(t.toolCount, len(tools)); err != nil {
			return err
		}
		if len(tools) > 0 {
			tool, _ := tools[0].(map[string]any)
			if _, ok := tool["parameters"]; !ok {
				return errf("offline: workers ai tool missing parameters")
			}
			if _, wrapped := tool["function"]; wrapped {
				return errf("offline: workers ai tool used an OpenAI function wrapper")
			}
		}
	default:
		return errf("offline: unsupported provider %q", t.provider)
	}
	return nil
}

func expectToolCount(want, got int) error {
	if want != got {
		return errf("offline: request listed %d tools, probe offered %d", got, want)
	}
	return nil
}

func countSlice(v any) (int, bool) {
	s, ok := v.([]any)
	if !ok {
		return 0, false
	}
	return len(s), true
}
