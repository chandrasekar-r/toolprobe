package score

import (
	"testing"
	"time"

	"github.com/chandrasekar-r/toolprobe/internal/probes"
)

func TestAggregate(t *testing.T) {
	results := []probes.ProbeResult{
		{Name: "a", Passed: true, Latency: 10 * time.Millisecond, LatencyMs: 10},
		{Name: "b", Passed: false, Latency: 20 * time.Millisecond, LatencyMs: 20},
	}
	s := Aggregate(results)
	if s.Total != 2 || s.Passed != 1 || s.Failed != 1 {
		t.Fatalf("counts: %+v", s)
	}
	if s.PassRate != 0.5 {
		t.Fatalf("pass_rate=%v", s.PassRate)
	}
	if s.AvgLatency != 15 {
		t.Fatalf("avg=%v", s.AvgLatency)
	}
}
