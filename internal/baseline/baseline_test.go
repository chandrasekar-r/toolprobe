package baseline

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chandrasekar-r/toolprobe/internal/probes"
	"github.com/chandrasekar-r/toolprobe/internal/report"
	"github.com/chandrasekar-r/toolprobe/internal/score"
)

func TestFromReportAggregatesRepeats(t *testing.T) {
	r := report.Report{
		GeneratedAt: time.Now().UTC(),
		Summary:     score.Summary{Total: 2, Passed: 2, Failed: 0, PassRate: 1},
		Results: []probes.ProbeResult{
			{Name: "weather#1", Passed: true},
			{Name: "weather#2", Passed: true},
		},
	}
	b := FromReport(r)
	st, ok := b.Probes["weather"]
	if !ok || st.Runs != 2 || st.PassRate != 1 {
		t.Fatalf("agg: %+v", b.Probes)
	}
}

func TestCompareDetectsRegression(t *testing.T) {
	base := Baseline{
		PassRate: 1.0,
		Probes: map[string]ProbeStat{
			"weather": {Runs: 1, Passed: 1, PassRate: 1},
		},
	}
	cur := report.Report{
		Summary: score.Summary{Total: 1, Passed: 0, Failed: 1, PassRate: 0},
		Results: []probes.ProbeResult{{Name: "weather", Passed: false}},
	}
	d := Compare(base, cur)
	if d.OK {
		t.Fatal("expected regression")
	}
	if len(d.Messages) == 0 {
		t.Fatal("expected messages")
	}
}

func TestComparePass(t *testing.T) {
	base := Baseline{
		PassRate: 0.5,
		Probes: map[string]ProbeStat{
			"a": {Runs: 2, Passed: 1, PassRate: 0.5},
		},
	}
	cur := report.Report{
		Summary: score.Summary{Total: 2, Passed: 2, Failed: 0, PassRate: 1},
		Results: []probes.ProbeResult{
			{Name: "a#1", Passed: true},
			{Name: "a#2", Passed: true},
		},
	}
	d := Compare(base, cur)
	if !d.OK {
		t.Fatalf("unexpected fail: %v", d.Messages)
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolprobe-baseline.json")
	b := Baseline{Version: 1, PassRate: 1, Probes: map[string]ProbeStat{"x": {Runs: 1, Passed: 1, PassRate: 1}}}
	if err := Save(path, b); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.PassRate != 1 || got.Probes["x"].Passed != 1 {
		t.Fatalf("%+v", got)
	}
}
