package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chandrasekar-r/toolprobe/internal/probes"
	"github.com/chandrasekar-r/toolprobe/internal/score"
)

func sampleReport() Report {
	return Report{
		GeneratedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		Model:       "mock",
		Mock:        true,
		Repeat:      1,
		MinPass:     1.0,
		Summary:     score.Summary{Total: 2, Passed: 1, Failed: 1, PassRate: 0.5, AvgLatency: 12},
		Results: []probes.ProbeResult{
			{Name: "a", Passed: true, LatencyMs: 4, ToolName: "get_weather", ToolArgs: `{"city":"Berlin"}`},
			{Name: "b", Passed: false, LatencyMs: 20, Error: "tool name mismatch"},
		},
	}
}

func TestWriteHTML(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteHTML(&buf, sampleReport()); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, want := range []string{"toolprobe scorecard", "PASS", "FAIL", "get_weather", "50%"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in html", want)
		}
	}
}

func TestSaveLoadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last.json")
	r := sampleReport()
	if err := SaveJSON(path, r); err != nil {
		t.Fatal(err)
	}
	got, err := LoadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary.Passed != 1 || got.Model != "mock" {
		t.Fatalf("roundtrip: %+v", got)
	}
	htmlPath := filepath.Join(dir, "scorecard.html")
	if err := SaveHTML(htmlPath, *got); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(htmlPath); err != nil {
		t.Fatal(err)
	}
}
