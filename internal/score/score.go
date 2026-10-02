// Package score aggregates probe results.
package score

import (
	"github.com/chandrasekar-r/toolprobe/internal/probes"
)

// Summary is an aggregate of probe results.
type Summary struct {
	Total      int     `json:"total"`
	Passed     int     `json:"passed"`
	Failed     int     `json:"failed"`
	PassRate   float64 `json:"pass_rate"`
	AvgLatency float64 `json:"avg_latency_ms"`
}

// Aggregate computes pass rate and average latency from results.
func Aggregate(results []probes.ProbeResult) Summary {
	s := Summary{Total: len(results)}
	var sumMs float64
	for _, r := range results {
		if r.Passed {
			s.Passed++
		} else {
			s.Failed++
		}
		sumMs += r.LatencyMs
	}
	if s.Total > 0 {
		s.PassRate = float64(s.Passed) / float64(s.Total)
		s.AvgLatency = sumMs / float64(s.Total)
	}
	return s
}
