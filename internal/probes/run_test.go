package probes

import (
	"context"
	"testing"

	"github.com/chandrasekar-r/toolprobe/internal/client"
)

func mockRunner() *Runner {
	c := client.New("", "")
	c.Mock = true
	return &Runner{Client: c, Model: "mock"}
}

func TestRunner_MockWeatherPass(t *testing.T) {
	r := mockRunner()
	p := &Probe{
		Name: "weather",
		User: "weather Berlin celsius",
		Tools: []ToolDef{{
			Name:       "get_weather",
			Parameters: map[string]interface{}{"type": "object"},
		}},
		Expect: Expect{
			ToolName: "get_weather",
			Args:     map[string]interface{}{"city": "Berlin", "units": "celsius"},
		},
		Mock: &MockSpec{
			Mode:     "tool_calls",
			ToolName: "get_weather",
			Args:     map[string]interface{}{"city": "Berlin", "units": "celsius"},
		},
	}
	res := r.Run(context.Background(), p)
	if !res.Passed {
		t.Fatalf("expected pass: %s", res.Error)
	}
}

func TestRunner_NoToolPass(t *testing.T) {
	r := mockRunner()
	p := &Probe{
		Name:  "refuse",
		User:  "book a flight",
		Tools: []ToolDef{{Name: "get_weather", Parameters: map[string]interface{}{"type": "object"}}},
		Expect: Expect{NoTool: true},
		Mock:   &MockSpec{Mode: "text", Content: "nope"},
	}
	res := r.Run(context.Background(), p)
	if !res.Passed {
		t.Fatalf("expected pass: %s", res.Error)
	}
}

func TestRunner_NoToolFailWhenToolCalled(t *testing.T) {
	r := mockRunner()
	p := &Probe{
		Name:  "refuse_fail",
		User:  "book",
		Tools: []ToolDef{{Name: "get_weather", Parameters: map[string]interface{}{"type": "object"}}},
		Expect: Expect{NoTool: true},
		Mock: &MockSpec{
			Mode:     "tool_calls",
			ToolName: "get_weather",
			Args:     map[string]interface{}{"city": "X"},
		},
	}
	res := r.Run(context.Background(), p)
	if res.Passed {
		t.Fatal("expected fail when tool called but no_tool expected")
	}
}

func TestRunner_WrongToolFail(t *testing.T) {
	r := mockRunner()
	p := &Probe{
		Name:  "wrong",
		User:  "weather",
		Tools: []ToolDef{{Name: "get_weather", Parameters: map[string]interface{}{"type": "object"}}},
		Expect: Expect{ToolName: "get_weather", Args: map[string]interface{}{"city": "Berlin"}},
		Mock:   &MockSpec{Mode: "wrong_tool", ToolName: "web_search"},
	}
	res := r.Run(context.Background(), p)
	if res.Passed {
		t.Fatal("expected fail on wrong tool")
	}
}

func TestRunner_EmptyArgsFail(t *testing.T) {
	r := mockRunner()
	p := &Probe{
		Name:  "empty",
		User:  "weather",
		Tools: []ToolDef{{Name: "get_weather", Parameters: map[string]interface{}{"type": "object"}}},
		Expect: Expect{
			ToolName: "get_weather",
			Args:     map[string]interface{}{"city": "Berlin", "units": "celsius"},
		},
		Mock: &MockSpec{Mode: "empty_args", ToolName: "get_weather"},
	}
	res := r.Run(context.Background(), p)
	if res.Passed {
		t.Fatal("expected fail on empty args")
	}
}

func TestRunner_MalformedArgsFail(t *testing.T) {
	r := mockRunner()
	p := &Probe{
		Name:  "malformed",
		User:  "weather",
		Tools: []ToolDef{{Name: "get_weather", Parameters: map[string]interface{}{"type": "object"}}},
		Expect: Expect{
			ToolName: "get_weather",
			Args:     map[string]interface{}{"city": "Berlin"},
		},
		Mock: &MockSpec{Mode: "malformed_args", ToolName: "get_weather"},
	}
	res := r.Run(context.Background(), p)
	if res.Passed {
		t.Fatal("expected fail on malformed args")
	}
}

func TestRunner_MultiCallsPass(t *testing.T) {
	r := mockRunner()
	p := &Probe{
		Name:  "parallel",
		User:  "Berlin and Paris",
		Tools: []ToolDef{{Name: "get_weather", Parameters: map[string]interface{}{"type": "object"}}},
		Expect: Expect{
			Calls: []ExpectCall{
				{ToolName: "get_weather", Args: map[string]interface{}{"city": "Berlin"}},
				{ToolName: "get_weather", Args: map[string]interface{}{"city": "Paris"}},
			},
		},
		Mock: &MockSpec{
			Mode: "tool_calls",
			ToolCalls: []MockToolCall{
				{Name: "get_weather", Args: map[string]interface{}{"city": "Paris", "units": "celsius"}},
				{Name: "get_weather", Args: map[string]interface{}{"city": "Berlin", "units": "celsius"}},
			},
		},
	}
	res := r.Run(context.Background(), p)
	if !res.Passed {
		t.Fatalf("expected pass (order-insensitive): %s", res.Error)
	}
}

func TestRunner_LatencyBudgetFail(t *testing.T) {
	r := mockRunner()
	p := &Probe{
		Name:  "slow",
		User:  "weather",
		Tools: []ToolDef{{Name: "get_weather", Parameters: map[string]interface{}{"type": "object"}}},
		Expect: Expect{
			ToolName:     "get_weather",
			Args:         map[string]interface{}{"city": "Berlin"},
			MaxLatencyMs: 5,
		},
		Mock: &MockSpec{
			Mode:      "tool_calls",
			LatencyMs: 30,
			ToolName:  "get_weather",
			Args:      map[string]interface{}{"city": "Berlin"},
		},
	}
	res := r.Run(context.Background(), p)
	if res.Passed {
		t.Fatal("expected fail when latency exceeds budget")
	}
}

func TestArgsMatchExtraKeys(t *testing.T) {
	got := map[string]interface{}{"city": "Berlin", "units": "celsius", "extra": true}
	want := map[string]interface{}{"city": "Berlin", "units": "celsius"}
	if !argsMatch(got, want) {
		t.Fatal("expected match with extra keys")
	}
}

func TestLoadDir_DefaultProbes(t *testing.T) {
	ps, err := LoadDir("../../probes/default")
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) < 10 {
		t.Fatalf("want >= 10 default probes, got %d", len(ps))
	}
	for _, p := range ps {
		r := mockRunner()
		res := r.Run(context.Background(), p)
		if !res.Passed {
			t.Errorf("default probe %s failed in mock: %s", p.Name, res.Error)
		}
	}
}
