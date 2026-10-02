// Package baseline stores and compares toolprobe pass rates for CI regression gates.
package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/chandrasekar-r/toolprobe/internal/probes"
	"github.com/chandrasekar-r/toolprobe/internal/report"
	"github.com/chandrasekar-r/toolprobe/internal/score"
)

// DefaultPath is the conventional baseline filename.
const DefaultPath = "toolprobe-baseline.json"

// ProbeStat is the recorded outcome for one probe name (aggregated across repeats).
type ProbeStat struct {
	Runs     int     `json:"runs"`
	Passed   int     `json:"passed"`
	PassRate float64 `json:"pass_rate"`
}

// Baseline is a durable snapshot used to detect regressions.
type Baseline struct {
	Version    int                  `json:"version"`
	Model      string               `json:"model,omitempty"`
	Mock       bool                 `json:"mock,omitempty"`
	PassRate   float64              `json:"pass_rate"`
	Total      int                  `json:"total"`
	Passed     int                  `json:"passed"`
	Failed     int                  `json:"failed"`
	AvgLatency float64              `json:"avg_latency_ms"`
	Probes     map[string]ProbeStat `json:"probes"`
}

// FromReport builds a baseline from a run report (aggregates #repeat suffixes).
func FromReport(r report.Report) Baseline {
	b := Baseline{
		Version:    1,
		Model:      r.Model,
		Mock:       r.Mock,
		PassRate:   r.Summary.PassRate,
		Total:      r.Summary.Total,
		Passed:     r.Summary.Passed,
		Failed:     r.Summary.Failed,
		AvgLatency: r.Summary.AvgLatency,
		Probes:     map[string]ProbeStat{},
	}
	for _, res := range r.Results {
		name := baseName(res.Name)
		st := b.Probes[name]
		st.Runs++
		if res.Passed {
			st.Passed++
		}
		b.Probes[name] = st
	}
	for name, st := range b.Probes {
		if st.Runs > 0 {
			st.PassRate = float64(st.Passed) / float64(st.Runs)
		}
		b.Probes[name] = st
	}
	return b
}

func baseName(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '#' {
			return name[:i]
		}
	}
	return name
}

// Save writes the baseline JSON.
func Save(path string, b Baseline) error {
	if dir := filepath.Dir(path); dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// Load reads a baseline JSON.
func Load(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	if b.Probes == nil {
		b.Probes = map[string]ProbeStat{}
	}
	return &b, nil
}

// Diff describes a regression against baseline.
type Diff struct {
	OK       bool     `json:"ok"`
	Messages []string `json:"messages,omitempty"`
}

// Compare checks current report against baseline.
// Fails if overall pass rate drops, or any probe's pass rate drops.
func Compare(base Baseline, current report.Report) Diff {
	cur := FromReport(current)
	var msgs []string
	if cur.PassRate+1e-9 < base.PassRate {
		msgs = append(msgs, fmt.Sprintf("pass_rate regression: got %.4f want >= %.4f", cur.PassRate, base.PassRate))
	}
	names := make([]string, 0, len(base.Probes))
	for n := range base.Probes {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		bst := base.Probes[name]
		cst, ok := cur.Probes[name]
		if !ok {
			msgs = append(msgs, fmt.Sprintf("probe %q missing from current run", name))
			continue
		}
		if cst.PassRate+1e-9 < bst.PassRate {
			msgs = append(msgs, fmt.Sprintf("probe %q pass_rate regression: got %.4f want >= %.4f", name, cst.PassRate, bst.PassRate))
		}
	}
	return Diff{OK: len(msgs) == 0, Messages: msgs}
}

// CompareResults is a convenience using raw results + summary.
func CompareResults(base Baseline, sum score.Summary, results []probes.ProbeResult) Diff {
	return Compare(base, report.Report{Summary: sum, Results: results})
}
