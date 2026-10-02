package probes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile_NoTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refuse.yaml")
	content := `
name: refuse
user: "do the impossible"
tools:
  - name: get_weather
    parameters: { type: object }
expect:
  no_tool: true
mock:
  mode: text
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Expect.NoTool {
		t.Fatal("expected no_tool")
	}
}

func TestLoadFile_RequiresExpect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	content := `
name: bad
user: "hi"
tools: []
expect: {}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil {
		t.Fatal("expected validation error")
	}
}
