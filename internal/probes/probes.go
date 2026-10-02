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
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	System      string            `yaml:"system,omitempty" json:"system,omitempty"`
	User        string            `yaml:"user" json:"user"`
	Tools       []ToolDef         `yaml:"tools" json:"tools"`
	Expect      Expect            `yaml:"expect" json:"expect"`
}

// ToolDef is a tool offered to the model (YAML form).
type ToolDef struct {
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Parameters  map[string]interface{} `yaml:"parameters" json:"parameters"`
}

// Expect asserts on the first tool call.
type Expect struct {
	ToolName string                 `yaml:"tool_name" json:"tool_name"`
	Args     map[string]interface{} `yaml:"args" json:"args"`
}

// ProbeResult is the outcome of running one probe.
type ProbeResult struct {
	Name       string        `json:"name"`
	Passed     bool          `json:"passed"`
	Latency    time.Duration `json:"latency_ns"`
	LatencyMs  float64       `json:"latency_ms"`
	Error      string        `json:"error,omitempty"`
	ToolName   string        `json:"tool_name,omitempty"`
	ToolArgs   string        `json:"tool_args,omitempty"`
	Expected   Expect        `json:"expected"`
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
	if p.Expect.ToolName == "" {
		return nil, fmt.Errorf("%s: expect.tool_name is required", path)
	}
	return &p, nil
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
