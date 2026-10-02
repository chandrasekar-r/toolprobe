package probes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "weather.yaml")
	content := `
name: weather_city_units
user: "Weather in Berlin, celsius please"
tools:
  - name: get_weather
    description: Get current weather
    parameters:
      type: object
      properties:
        city:
          type: string
        units:
          type: string
          enum: [celsius, fahrenheit]
      required: [city, units]
expect:
  tool_name: get_weather
  args:
    city: Berlin
    units: celsius
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "weather_city_units" {
		t.Fatalf("name=%q", p.Name)
	}
	if p.Expect.ToolName != "get_weather" {
		t.Fatalf("expect tool=%q", p.Expect.ToolName)
	}
	if p.Expect.Args["city"] != "Berlin" {
		t.Fatalf("args=%#v", p.Expect.Args)
	}
}

func TestLoadDir(t *testing.T) {
	// Load the repo default probe if present relative to module.
	root := filepath.Join("..", "..", "probes", "default")
	if _, err := os.Stat(root); err != nil {
		t.Skip("default probes not present")
	}
	ps, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) < 1 {
		t.Fatal("expected at least one probe")
	}
}
