// Package report writes JSON/HTML run reports and persists the last run.
package report

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chandrasekar-r/toolprobe/internal/probes"
	"github.com/chandrasekar-r/toolprobe/internal/score"
)

const (
	// DefaultLastPath is where `run` stores the latest JSON report.
	DefaultLastPath = ".toolprobe/last-report.json"
	// DefaultScorecardPath is the default HTML output.
	DefaultScorecardPath = "scorecard.html"
)

// Report is the full JSON output of a toolprobe run.
type Report struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Model       string               `json:"model"`
	Mock        bool                 `json:"mock"`
	Repeat      int                  `json:"repeat"`
	MinPass     float64              `json:"min_pass"`
	Summary     score.Summary        `json:"summary"`
	Results     []probes.ProbeResult `json:"results"`
}

// WriteJSON encodes the report to w with indentation.
func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	return nil
}

// SaveJSON writes the report to path (creating parent dirs).
func SaveJSON(path string, r Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteJSON(f, r)
}

// LoadJSON reads a report from path.
func LoadJSON(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse report: %w", err)
	}
	return &r, nil
}

// WriteHTML writes a static scorecard HTML page.
func WriteHTML(w io.Writer, r Report) error {
	passPct := r.Summary.PassRate * 100
	statusClass := "ok"
	statusLabel := "PASS"
	if r.Summary.Failed > 0 || r.Summary.PassRate < r.MinPass {
		statusClass = "fail"
		statusLabel = "FAIL"
	}

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>toolprobe scorecard</title>
<style>
:root { --bg:#0f1419; --card:#1a2332; --text:#e7ecf3; --muted:#8b9bb4; --ok:#3dd68c; --fail:#f07178; --line:#2a3548; }
* { box-sizing: border-box; }
body { margin:0; font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, sans-serif; background:var(--bg); color:var(--text); line-height:1.45; }
main { max-width: 960px; margin: 0 auto; padding: 2rem 1.25rem 3rem; }
h1 { font-size: 1.5rem; font-weight: 650; margin: 0 0 .25rem; }
.sub { color: var(--muted); font-size: .9rem; margin-bottom: 1.5rem; }
.cards { display:grid; grid-template-columns: repeat(auto-fit,minmax(140px,1fr)); gap: .75rem; margin-bottom: 1.5rem; }
.card { background: var(--card); border: 1px solid var(--line); border-radius: 10px; padding: 1rem; }
.card .label { color: var(--muted); font-size: .75rem; text-transform: uppercase; letter-spacing: .04em; }
.card .value { font-size: 1.4rem; font-weight: 650; margin-top: .2rem; }
.badge { display:inline-block; padding: .15rem .55rem; border-radius: 999px; font-size: .75rem; font-weight: 650; }
.badge.ok { background: color-mix(in srgb, var(--ok) 20%, transparent); color: var(--ok); }
.badge.fail { background: color-mix(in srgb, var(--fail) 20%, transparent); color: var(--fail); }
table { width:100%; border-collapse: collapse; background: var(--card); border: 1px solid var(--line); border-radius: 10px; overflow: hidden; }
th, td { text-align: left; padding: .65rem .8rem; border-bottom: 1px solid var(--line); font-size: .9rem; }
th { color: var(--muted); font-weight: 600; font-size: .75rem; text-transform: uppercase; letter-spacing: .03em; }
tr:last-child td { border-bottom: none; }
.err { color: var(--fail); font-size: .8rem; }
.foot { margin-top: 1.25rem; color: var(--muted); font-size: .8rem; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: .85em; }
</style>
</head>
<body>
<main>
<h1>toolprobe scorecard <span class="badge `)
	b.WriteString(statusClass)
	b.WriteString(`">`)
	b.WriteString(statusLabel)
	b.WriteString(`</span></h1>
<p class="sub">CI for tool calling — model <code>`)
	b.WriteString(html.EscapeString(r.Model))
	b.WriteString(`</code>`)
	if r.Mock {
		b.WriteString(` · <code>mock</code>`)
	}
	b.WriteString(` · generated `)
	b.WriteString(html.EscapeString(r.GeneratedAt.UTC().Format(time.RFC3339)))
	b.WriteString(`</p>
<div class="cards">
  <div class="card"><div class="label">Pass rate</div><div class="value">`)
	b.WriteString(fmt.Sprintf("%.0f%%", passPct))
	b.WriteString(`</div></div>
  <div class="card"><div class="label">Passed</div><div class="value">`)
	b.WriteString(fmt.Sprintf("%d / %d", r.Summary.Passed, r.Summary.Total))
	b.WriteString(`</div></div>
  <div class="card"><div class="label">Failed</div><div class="value">`)
	b.WriteString(fmt.Sprintf("%d", r.Summary.Failed))
	b.WriteString(`</div></div>
  <div class="card"><div class="label">Avg latency</div><div class="value">`)
	b.WriteString(fmt.Sprintf("%.1f ms", r.Summary.AvgLatency))
	b.WriteString(`</div></div>
  <div class="card"><div class="label">Min pass</div><div class="value">`)
	b.WriteString(fmt.Sprintf("%.0f%%", r.MinPass*100))
	b.WriteString(`</div></div>
  <div class="card"><div class="label">Repeat</div><div class="value">`)
	b.WriteString(fmt.Sprintf("%d", r.Repeat))
	b.WriteString(`</div></div>
</div>
<table>
<thead><tr><th>Status</th><th>Probe</th><th>Latency</th><th>Tool</th><th>Detail</th></tr></thead>
<tbody>
`)
	for _, res := range r.Results {
		st := "ok"
		lab := "PASS"
		if !res.Passed {
			st = "fail"
			lab = "FAIL"
		}
		b.WriteString(`<tr><td><span class="badge `)
		b.WriteString(st)
		b.WriteString(`">`)
		b.WriteString(lab)
		b.WriteString(`</span></td><td><code>`)
		b.WriteString(html.EscapeString(res.Name))
		b.WriteString(`</code></td><td>`)
		b.WriteString(fmt.Sprintf("%.1f ms", res.LatencyMs))
		b.WriteString(`</td><td><code>`)
		b.WriteString(html.EscapeString(res.ToolName))
		b.WriteString(`</code></td><td>`)
		if res.Error != "" {
			b.WriteString(`<span class="err">`)
			b.WriteString(html.EscapeString(res.Error))
			b.WriteString(`</span>`)
		} else if res.ToolArgs != "" {
			b.WriteString(`<code>`)
			b.WriteString(html.EscapeString(truncate(res.ToolArgs, 80)))
			b.WriteString(`</code>`)
		} else {
			b.WriteString(`—`)
		}
		b.WriteString(`</td></tr>
`)
	}
	b.WriteString(`</tbody></table>
<p class="foot">Generated by <a href="https://github.com/chandrasekar-r/toolprobe" style="color:var(--muted)">toolprobe</a>. Static HTML — no JS required.</p>
</main>
</body>
</html>
`)
	_, err := io.WriteString(w, b.String())
	return err
}

// SaveHTML writes the scorecard to path.
func SaveHTML(path string, r Report) error {
	if dir := filepath.Dir(path); dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteHTML(f, r)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
