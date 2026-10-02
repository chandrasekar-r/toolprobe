package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/chandrasekar-r/toolprobe/internal/client"
	"github.com/chandrasekar-r/toolprobe/internal/probes"
	"github.com/chandrasekar-r/toolprobe/internal/report"
	"github.com/chandrasekar-r/toolprobe/internal/score"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var (
	flagBaseURL string
	flagAPIKey  string
	flagModel   string
	flagJSON    bool
	flagMock    bool
	flagProbes  string
	flagTimeout time.Duration
	flagRepeat  int
	flagMinPass float64
)

var rootCmd = &cobra.Command{
	Use:   "toolprobe",
	Short: "CI for LLM tool calling — probe OpenAI-compatible APIs",
	Long:  "toolprobe runs YAML probes against OpenAI-compatible chat/completions endpoints and asserts correct tool names and JSON arguments.",
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run tool-calling probes",
	RunE:  runProbes,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&flagBaseURL, "base-url", envOr("TOOLPROBE_BASE_URL", ""), "OpenAI-compatible API base URL (e.g. https://api.openai.com/v1)")
	runCmd.Flags().StringVar(&flagAPIKey, "api-key", envOr("TOOLPROBE_API_KEY", ""), "API key (or TOOLPROBE_API_KEY)")
	runCmd.Flags().StringVar(&flagModel, "model", envOr("TOOLPROBE_MODEL", "gpt-4o-mini"), "Model name")
	runCmd.Flags().BoolVar(&flagJSON, "json", false, "Emit JSON report to stdout")
	runCmd.Flags().BoolVar(&flagMock, "mock", false, "Dry-run / mock mode: no network; uses probe.mock")
	runCmd.Flags().StringVar(&flagProbes, "probes", "probes/default", "Directory of probe YAML files")
	runCmd.Flags().DurationVar(&flagTimeout, "timeout", 60*time.Second, "Per-probe timeout")
	runCmd.Flags().IntVar(&flagRepeat, "repeat", 1, "Run each probe N times (for flaky live APIs)")
	runCmd.Flags().Float64Var(&flagMinPass, "min-pass", 1.0, "Minimum pass rate (0-1) required to exit 0")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func runProbes(cmd *cobra.Command, args []string) error {
	if flagRepeat < 1 {
		return fmt.Errorf("--repeat must be >= 1")
	}
	if flagMinPass < 0 || flagMinPass > 1 {
		return fmt.Errorf("--min-pass must be between 0 and 1")
	}

	probeDir := flagProbes
	if !filepath.IsAbs(probeDir) {
		if abs, err := filepath.Abs(probeDir); err == nil {
			probeDir = abs
		}
	}

	loaded, err := probes.LoadDir(probeDir)
	if err != nil {
		return err
	}

	c := client.New(flagBaseURL, flagAPIKey)
	c.Mock = flagMock
	runner := &probes.Runner{Client: c, Model: flagModel}

	ctx := context.Background()
	results := make([]probes.ProbeResult, 0, len(loaded)*flagRepeat)
	for i := 0; i < flagRepeat; i++ {
		for _, p := range loaded {
			pctx, cancel := context.WithTimeout(ctx, flagTimeout)
			res := runner.Run(pctx, p)
			cancel()
			if flagRepeat > 1 {
				res.Name = fmt.Sprintf("%s#%d", p.Name, i+1)
			}
			results = append(results, res)
			if !flagJSON {
				status := "PASS"
				if !res.Passed {
					status = "FAIL"
				}
				fmt.Fprintf(os.Stderr, "[%s] %s (%.1fms)", status, res.Name, res.LatencyMs)
				if res.Error != "" {
					fmt.Fprintf(os.Stderr, " — %s", res.Error)
				}
				fmt.Fprintln(os.Stderr)
			}
		}
	}

	sum := score.Aggregate(results)
	rep := report.Report{
		GeneratedAt: time.Now().UTC(),
		Model:       flagModel,
		Mock:        flagMock,
		Repeat:      flagRepeat,
		MinPass:     flagMinPass,
		Summary:     sum,
		Results:     results,
	}

	if flagJSON {
		if err := report.WriteJSON(os.Stdout, rep); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(os.Stderr, "\n%d/%d passed (%.0f%%)  avg latency %.1fms  min-pass %.0f%%\n",
			sum.Passed, sum.Total, sum.PassRate*100, sum.AvgLatency, flagMinPass*100)
	}

	if sum.PassRate < flagMinPass {
		os.Exit(1)
	}
	return nil
}
