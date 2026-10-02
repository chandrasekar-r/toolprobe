// Package probes loads and describes tool-calling probes from YAML.
package probes

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Probe is a single tool-calling reliability check.
type Probe struct {
	Name        string    `yaml:"name" json:"name"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	System      string    `yaml:"system,omitempty" json:"system,omitempty"`
	User        string    `yaml:"user" json:"user"`
	Tools       []ToolDef `yaml:"tools" json:"tools"`
	Expect      Expect    `yaml:"expect" json:"expect"`
	// Mock drives --mock responses. Required for deterministic CI when using mock mode.
	Mock *MockSpec `yaml:"mock,omitempty" json:"mock,omitempty"`
}

// ToolDef is a tool offered to the model (YAML form).
type ToolDef struct {
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Parameters  map[string]interface{} `yaml:"parameters" json:"parameters"`
}

// ExpectCall is one expected tool invocation (for multi-tool / parallel).
type ExpectCall struct {
	ToolName string                 `yaml:"tool_name" json:"tool_name"`
	Args     map[string]interface{} `yaml:"args,omitempty" json:"args,omitempty"`
}

// Expect asserts on the model response.
type Expect struct {
	// ToolName + Args: single tool call (first call). Ignored if NoTool or Calls set.
	ToolName string                 `yaml:"tool_name,omitempty" json:"tool_name,omitempty"`
	Args     map[string]interface{} `yaml:"args,omitempty" json:"args,omitempty"`
	// Calls: expect multiple tool calls (order-insensitive match by name+args subset).
	Calls []ExpectCall `yaml:"calls,omitempty" json:"calls,omitempty"`
	// NoTool: expect the model to refuse / answer without tool calls.
	NoTool bool `yaml:"no_tool,omitempty" json:"no_tool,omitempty"`
	// MaxLatencyMs: fail if observed latency exceeds this (mockable via mock.latency_ms).
	MaxLatencyMs float64 `yaml:"max_latency_ms,omitempty" json:"max_latency_ms,omitempty"`
}

// MockToolCall is a canned tool call for --mock.
type MockToolCall struct {
	Name      string                 `yaml:"name" json:"name"`
	Arguments string                 `yaml:"arguments,omitempty" json:"arguments,omitempty"` // raw JSON
	Args      map[string]interface{} `yaml:"args,omitempty" json:"args,omitempty"`
}

// MockSpec describes how --mock synthesizes a chat response for this probe.
type MockSpec struct {
	// Mode: "tool_calls" (default), "text", "empty_args", "malformed_args", "wrong_tool"
	Mode      string         `yaml:"mode,omitempty" json:"mode,omitempty"`
	LatencyMs int            `yaml:"latency_ms,omitempty" json:"latency_ms,omitempty"`
	Content   string         `yaml:"content,omitempty" json:"content,omitempty"`
	ToolCalls []MockToolCall `yaml:"tool_calls,omitempty" json:"tool_calls,omitempty"`
	// Shorthand for a single tool call (merged into ToolCalls if ToolCalls empty).
	ToolName string                 `yaml:"tool_name,omitempty" json:"tool_name,omitempty"`
	Args     map[string]interface{} `yaml:"args,omitempty" json:"args,omitempty"`
}

// ProbeResult is the outcome of running one probe.
type ProbeResult struct {
	Name      string        `json:"name"`
	Passed    bool          `json:"passed"`
	Latency   time.Duration `json:"latency_ns"`
	LatencyMs float64       `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
	ToolName  string        `json:"tool_name,omitempty"`
	ToolArgs  string        `json:"tool_args,omitempty"`
	Expected  Expect        `json:"expected"`
}

// LoadFile reads a single probe YAML file.
func LoadFile(path string) (*Probe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Probe
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if p.Name == "" {
		p.Name = filepath.Base(path)
	}
	if p.User == "" {
		return nil, fmt.Errorf("%s: user message is required", path)
	}
	if err := validateExpect(path, &p.Expect); err != nil {
		return nil, err
	}
	return &p, nil
}

func validateExpect(path string, e *Expect) error {
	if e.NoTool {
		return nil
	}
	if len(e.Calls) > 0 {
		for i, c := range e.Calls {
			if c.ToolName == "" {
				return fmt.Errorf("%s: expect.calls[%d].tool_name is required", path, i)
			}
		}
		return nil
	}
	if e.ToolName == "" {
		return fmt.Errorf("%s: expect.tool_name is required (or expect.no_tool / expect.calls)", path)
	}
	return nil
}

// LoadDir loads all *.yaml / *.yml probes from a directory (non-recursive).
func LoadDir(dir string) ([]*Probe, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []*Probe
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		p, err := LoadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no probe YAML files in %s", dir)
	}
	return out, nil
}
