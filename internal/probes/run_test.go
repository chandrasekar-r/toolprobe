package probes

import (
	"context"
	"testing"

	"github.com/chandrasekar-r/toolprobe/internal/client"
)

func TestRunner_MockWeather(t *testing.T) {
	c := client.New("", "")
	c.Mock = true
	r := &Runner{Client: c, Model: "mock"}

	p := &Probe{
		Name: "weather_city_units",
		User: "What is the weather in Berlin in celsius?",
		Tools: []ToolDef{{
			Name:        "get_weather",
			Description: "Get weather for a city",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"city":  map[string]interface{}{"type": "string"},
					"units": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"city", "units"},
			},
		}},
		Expect: Expect{
			ToolName: "get_weather",
			Args: map[string]interface{}{
				"city":  "Berlin",
				"units": "celsius",
			},
		},
	}

	res := r.Run(context.Background(), p)
	if !res.Passed {
		t.Fatalf("expected pass, got fail: %s", res.Error)
	}
}

func TestArgsMatchExtraKeys(t *testing.T) {
	got := map[string]interface{}{"city": "Berlin", "units": "celsius", "extra": true}
	want := map[string]interface{}{"city": "Berlin", "units": "celsius"}
	if !argsMatch(got, want) {
		t.Fatal("expected match with extra keys")
	}
}
