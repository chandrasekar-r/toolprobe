// Package report writes JSON run reports.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/chandrasekar-r/toolprobe/internal/probes"
	"github.com/chandrasekar-r/toolprobe/internal/score"
)

// Report is the full JSON output of a toolprobe run.
type Report struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Model       string               `json:"model"`
	Mock        bool                 `json:"mock"`
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
