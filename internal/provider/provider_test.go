package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonical(t *testing.T) {
	cases := map[string]string{
		"":           OpenAI,
		"openai":     OpenAI,
		"claude":     Anthropic,
		"anthropic":  Anthropic,
		"google":     Gemini,
		"gemini":     Gemini,
		"workers-ai": Cloudflare,
		"cloudflare": Cloudflare,
		"Workers_AI": Cloudflare,
	}
	for in, want := range cases {
		got, err := Canonical(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got != want {
			t.Fatalf("%q: got %s want %s", in, got, want)
		}
	}
	if _, err := Canonical("bedrock"); err == nil {
		t.Fatal("expected unknown provider error")
	}
}

func TestResolveAPIKeyPrecedence(t *testing.T) {
	t.Setenv("TOOLPROBE_API_KEY", "shared")
	t.Setenv("ANTHROPIC_API_KEY", "anth")
	t.Setenv("GEMINI_API_KEY", "gem")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cf")
	t.Setenv("OPENAI_API_KEY", "oai")

	if got := ResolveAPIKey(Anthropic, "", false); got != "anth" {
		t.Fatalf("anthropic key: %q", got)
	}
	if got := ResolveAPIKey(Gemini, "", false); got != "gem" {
		t.Fatalf("gemini key: %q", got)
	}
	if got := ResolveAPIKey(Cloudflare, "", false); got != "cf" {
		t.Fatalf("cloudflare key: %q", got)
	}
	if got := ResolveAPIKey(OpenAI, "", false); got != "shared" {
		t.Fatalf("openai prefers TOOLPROBE_API_KEY, got %q", got)
	}
	if got := ResolveAPIKey(Anthropic, "flag", true); got != "flag" {
		t.Fatalf("explicit flag should win, got %q", got)
	}

	os.Unsetenv("TOOLPROBE_API_KEY")
	if got := ResolveAPIKey(OpenAI, "", false); got != "oai" {
		t.Fatalf("openai fallback OPENAI_API_KEY: %q", got)
	}
}

func TestResolveModel(t *testing.T) {
	got, err := ResolveModel(OpenAI, "", false, false)
	if err != nil || got != "gpt-4o-mini" {
		t.Fatalf("openai default: %q %v", got, err)
	}
	if _, err := ResolveModel(Gemini, "", false, false); err == nil {
		t.Fatal("gemini live run must require --model")
	}
	got, err = ResolveModel(Gemini, "", false, true)
	if err != nil || got != "" {
		t.Fatalf("gemini mock may omit model: %q %v", got, err)
	}
	got, err = ResolveModel(Anthropic, "claude-sonnet-4-5", true, false)
	if err != nil || got != "claude-sonnet-4-5" {
		t.Fatalf("explicit model: %q %v", got, err)
	}
}

func TestNewRequiresCreds(t *testing.T) {
	if _, err := New(Config{Provider: OpenAI}); err == nil {
		t.Fatal("openai requires base-url")
	}
	if _, err := New(Config{Provider: Anthropic, BaseURL: "https://api.anthropic.com"}); err == nil {
		t.Fatal("anthropic requires key")
	}
	if _, err := New(Config{Provider: Gemini}); err == nil {
		t.Fatal("gemini requires key")
	}
	if _, err := New(Config{Provider: Cloudflare, APIKey: "tok"}); err == nil {
		t.Fatal("cloudflare requires account id")
	}
}

func TestOpenAIWire(t *testing.T) {
	var sawPath, sawAuth string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(mustRead(t, "openai_tool_call.json"))
	}))
	defer srv.Close()

	p, err := New(Config{Provider: OpenAI, BaseURL: srv.URL + "/v1", APIKey: "sk-test", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Complete(context.Background(), sampleTurn())
	if err != nil {
		t.Fatal(err)
	}
	if sawPath != "/v1/chat/completions" {
		t.Fatalf("path %s", sawPath)
	}
	if sawAuth != "Bearer sk-test" {
		t.Fatalf("auth %s", sawAuth)
	}
	tools := body["tools"].([]any)
	fn := tools[0].(map[string]any)["function"].(map[string]any)
	if fn["name"] != "get_weather" {
		t.Fatalf("tool dropped: %#v", body["tools"])
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("calls %#v", res.ToolCalls)
	}
	assertArgs(t, res.ToolCalls[0].Arguments, "Berlin", "celsius")
}

func TestAnthropicWire(t *testing.T) {
	var sawPath, sawKey, sawVersion string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.String()
		sawKey = r.Header.Get("x-api-key")
		sawVersion = r.Header.Get("anthropic-version")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write(mustRead(t, "anthropic_tool_use.json"))
	}))
	defer srv.Close()

	p, err := New(Config{Provider: Anthropic, BaseURL: srv.URL, APIKey: "ak-test", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Complete(context.Background(), sampleTurn())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(sawPath, "/v1/messages") {
		t.Fatalf("path %s", sawPath)
	}
	if strings.Contains(sawPath, "chat/completions") {
		t.Fatal("anthropic must not use chat/completions")
	}
	if sawKey != "ak-test" || sawVersion != AnthropicVersion {
		t.Fatalf("headers key=%s version=%s", sawKey, sawVersion)
	}
	if _, ok := body["input_schema"]; ok {
		t.Fatal("input_schema belongs on the tool, not the request root")
	}
	tool := body["tools"].([]any)[0].(map[string]any)
	if _, ok := tool["input_schema"]; !ok {
		t.Fatalf("missing input_schema: %#v", tool)
	}
	if _, ok := tool["function"]; ok {
		t.Fatal("anthropic tool must not use OpenAI function wrapper")
	}
	if body["system"] == "" {
		t.Fatal("system prompt should be top-level")
	}
	for _, raw := range body["messages"].([]any) {
		if raw.(map[string]any)["role"] == "system" {
			t.Fatal("system must not be a message role")
		}
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].ID != "toolu_01" {
		t.Fatalf("calls %#v", res.ToolCalls)
	}
	assertArgs(t, res.ToolCalls[0].Arguments, "Berlin", "celsius")
}

func TestAnthropicParallelFixture(t *testing.T) {
	srv := fixtureServer(t, "anthropic_parallel.json")
	defer srv.Close()
	p, err := New(Config{Provider: Anthropic, BaseURL: srv.URL, APIKey: "k", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Complete(context.Background(), sampleTurn())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 2 {
		t.Fatalf("want 2 calls, got %#v", res.ToolCalls)
	}
}

func TestGeminiWire(t *testing.T) {
	var sawPath, sawKey, sawQuery string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawQuery = r.URL.RawQuery
		sawKey = r.Header.Get("x-goog-api-key")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write(mustRead(t, "gemini_function_call.json"))
	}))
	defer srv.Close()

	p, err := New(Config{Provider: Gemini, BaseURL: srv.URL, APIKey: "gk", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	turn := sampleTurn()
	turn.Model = "gemini-3.8-flash"
	res, err := p.Complete(context.Background(), turn)
	if err != nil {
		t.Fatal(err)
	}
	if sawPath != "/v1beta/models/gemini-3.8-flash:generateContent" {
		t.Fatalf("path %s", sawPath)
	}
	if strings.Contains(sawPath, "chat/completions") || sawQuery != "" {
		t.Fatalf("path %s query %s", sawPath, sawQuery)
	}
	if sawKey != "gk" {
		t.Fatalf("key header %s", sawKey)
	}
	tools := body["tools"].([]any)[0].(map[string]any)
	decls := tools["functionDeclarations"].([]any)
	if decls[0].(map[string]any)["name"] != "get_weather" {
		t.Fatalf("declarations %#v", decls)
	}
	if _, ok := body["messages"]; ok {
		t.Fatal("gemini generateContent uses contents, not chat messages")
	}
	sys := body["systemInstruction"].(map[string]any)
	if sys == nil {
		t.Fatal("missing systemInstruction")
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("calls %#v", res.ToolCalls)
	}
	assertArgs(t, res.ToolCalls[0].Arguments, "Berlin", "celsius")
}

func TestGeminiRequiresExplicitModel(t *testing.T) {
	p, err := New(Config{Provider: Gemini, APIKey: "k", HTTPClient: http.DefaultClient})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Complete(context.Background(), Turn{User: "hi"})
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Fatalf("got %v", err)
	}
}

func TestGeminiParallelFixture(t *testing.T) {
	srv := fixtureServer(t, "gemini_parallel.json")
	defer srv.Close()
	p, err := New(Config{Provider: Gemini, BaseURL: srv.URL, APIKey: "k", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Complete(context.Background(), Turn{Model: "gemini-3.8-flash", User: "both"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 2 {
		t.Fatalf("want 2, got %#v", res.ToolCalls)
	}
}

func TestCloudflareWire(t *testing.T) {
	var sawPath, sawAuth string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write(mustRead(t, "cloudflare_tool_calls.json"))
	}))
	defer srv.Close()

	p, err := New(Config{
		Provider:   Cloudflare,
		BaseURL:    srv.URL + "/client/v4",
		APIKey:     "cf-token",
		AccountID:  "acct_123",
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	turn := sampleTurn()
	turn.Model = "@cf/meta/llama-4-scout-17b-16e-instruct"
	res, err := p.Complete(context.Background(), turn)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := "/client/v4/accounts/acct_123/ai/run/@cf/meta/llama-4-scout-17b-16e-instruct"
	if sawPath != wantPath {
		t.Fatalf("path %s", sawPath)
	}
	if strings.Contains(sawPath, "chat/completions") {
		t.Fatal("workers ai must not use chat/completions")
	}
	if sawAuth != "Bearer cf-token" {
		t.Fatalf("auth %s", sawAuth)
	}
	tool := body["tools"].([]any)[0].(map[string]any)
	if tool["name"] != "get_weather" {
		t.Fatalf("tool %#v", tool)
	}
	if _, ok := tool["function"]; ok || tool["type"] == "function" {
		t.Fatal("traditional workers ai tools are {name,description,parameters}, not OpenAI wrappers")
	}
	msgs := body["messages"].([]any)
	if msgs[0].(map[string]any)["role"] != "system" {
		t.Fatal("expected system message")
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("calls %#v", res.ToolCalls)
	}
	assertArgs(t, res.ToolCalls[0].Arguments, "Berlin", "celsius")
}

func TestCloudflareParallelAndTextFixtures(t *testing.T) {
	srv := fixtureServer(t, "cloudflare_parallel.json")
	defer srv.Close()
	p, err := New(Config{Provider: Cloudflare, BaseURL: srv.URL, APIKey: "k", AccountID: "a", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Complete(context.Background(), Turn{Model: "@cf/example/tool-model", User: "both", Tools: sampleTurn().Tools})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 2 {
		t.Fatalf("parallel %#v", res.ToolCalls)
	}

	textSrv := fixtureServer(t, "cloudflare_text.json")
	defer textSrv.Close()
	p, err = New(Config{Provider: Cloudflare, BaseURL: textSrv.URL, APIKey: "k", AccountID: "a", HTTPClient: textSrv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	res, err = p.Complete(context.Background(), Turn{Model: "@cf/example/tool-model", User: "book a flight", Tools: sampleTurn().Tools})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 0 {
		t.Fatalf("text fixture should have no calls: %#v", res.ToolCalls)
	}
	if res.Warning != "" {
		t.Fatalf("empty tool_calls is an acknowledgement, warning=%q", res.Warning)
	}
}

func TestCloudflareUnsupportedFailsClearly(t *testing.T) {
	for _, fixture := range []string{"cloudflare_unsupported_string.json", "cloudflare_unsupported_error.json"} {
		t.Run(fixture, func(t *testing.T) {
			srv := fixtureServer(t, fixture)
			defer srv.Close()
			p, err := New(Config{Provider: Cloudflare, BaseURL: srv.URL, APIKey: "k", AccountID: "a", HTTPClient: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = p.Complete(context.Background(), sampleTurn())
			if err == nil {
				t.Fatal("expected unsupported-tools error")
			}
			msg := err.Error()
			if !strings.Contains(msg, "does not support") && !strings.Contains(msg, "not support") {
				t.Fatalf("error should say the model does not support tools: %s", msg)
			}
			if strings.Contains(msg, "dropped") && strings.Contains(msg, "were dropped") {
				t.Fatalf("must not claim tools were dropped: %s", msg)
			}
		})
	}
}

func TestCloudflareOmittedToolCallsWarns(t *testing.T) {
	srv := fixtureServer(t, "cloudflare_omitted_tool_calls.json")
	defer srv.Close()
	p, err := New(Config{Provider: Cloudflare, BaseURL: srv.URL, APIKey: "k", AccountID: "a", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Complete(context.Background(), sampleTurn())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 0 || res.Warning == "" {
		t.Fatalf("want warning and no calls, got %#v", res)
	}
}

func TestURLs(t *testing.T) {
	u, err := geminiGenerateURL("https://generativelanguage.googleapis.com/", "models/gemini-3.8-flash")
	if err != nil || u != "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.8-flash:generateContent" {
		t.Fatalf("gemini url %s %v", u, err)
	}
	if got := anthropicMessagesURL("https://api.anthropic.com/v1"); got != "https://api.anthropic.com/v1/messages" {
		t.Fatal(got)
	}
	cu, err := cloudflareRunURL("https://api.cloudflare.com/client/v4/", "acc/id", "@cf/meta/llama-4-scout-17b-16e-instruct")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cu, "/accounts/acc%2Fid/ai/run/@cf/meta/llama-4-scout-17b-16e-instruct") {
		t.Fatal(cu)
	}
}

func sampleTurn() Turn {
	return Turn{
		Model:  "gpt-4o-mini",
		System: "Use tools.",
		User:   "Weather in Berlin in celsius?",
		Tools: []Tool{{
			Name:        "get_weather",
			Description: "Get weather",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city":  map[string]any{"type": "string"},
					"units": map[string]any{"type": "string"},
				},
				"required": []any{"city", "units"},
			},
		}},
	}
}

func assertArgs(t *testing.T, raw, city, units string) {
	t.Helper()
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatal(err)
	}
	if args["city"] != city || args["units"] != units {
		t.Fatalf("args %s", raw)
	}
}

func fixtureServer(t *testing.T, name string) *httptest.Server {
	t.Helper()
	body := mustRead(t, name)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
