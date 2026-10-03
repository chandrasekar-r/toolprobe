package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/chandrasekar-r/toolprobe/internal/baseline"
	"github.com/chandrasekar-r/toolprobe/internal/client"
	"github.com/chandrasekar-r/toolprobe/internal/probes"
	"github.com/chandrasekar-r/toolprobe/internal/provider"
	"github.com/chandrasekar-r/toolprobe/internal/report"
	"github.com/chandrasekar-r/toolprobe/internal/score"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var (
	flagProvider       string
	flagBaseURL        string
	flagAPIKey         string
	flagAccountID      string
	flagModel          string
	flagJSON           bool
	flagMock           bool
	flagProbes         string
	flagTimeout        time.Duration
	flagRepeat         int
	flagMinPass        float64
	flagBaseline       string
	flagWriteBaseline  string
	flagHTML           string
	flagLastPath       string
	flagReportOut      string
	flagReportLast     bool
	flagReportLastPath string
)

var rootCmd = &cobra.Command{
	Use:   "toolprobe",
	Short: "CI for LLM tool calling — probe OpenAI, Anthropic, Gemini, and Workers AI",
	Long: `toolprobe runs YAML probes against native tool-calling APIs and asserts correct tool names and JSON arguments.

Providers: openai (chat/completions), anthropic (Messages tool use), gemini (generateContent function calling), cloudflare (Workers AI /ai/run). API keys come from flags or environment variables. --mock never calls the network.`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run tool-calling probes",
	RunE:  runProbes,
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Render HTML scorecard from a saved JSON report",
	RunE:  runReport,
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(reportCmd)

	runCmd.Flags().StringVar(&flagProvider, "provider", envOr("TOOLPROBE_PROVIDER", provider.OpenAI), "Provider: openai, anthropic, gemini, cloudflare (aliases: claude, google, workers-ai)")
	runCmd.Flags().StringVar(&flagBaseURL, "base-url", envOr("TOOLPROBE_BASE_URL", ""), "API origin. OpenAI example: https://api.openai.com/v1. Other providers default to their public origin when omitted")
	runCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key. Overrides provider env vars. Defaults: TOOLPROBE_API_KEY, or ANTHROPIC_API_KEY / GEMINI_API_KEY / CLOUDFLARE_API_TOKEN")
	runCmd.Flags().StringVar(&flagAccountID, "account-id", "", "Cloudflare account id (or CLOUDFLARE_ACCOUNT_ID / TOOLPROBE_CLOUDFLARE_ACCOUNT_ID)")
	runCmd.Flags().StringVar(&flagModel, "model", envOr("TOOLPROBE_MODEL", ""), "Model id. OpenAI defaults to gpt-4o-mini. Required for anthropic, gemini, and cloudflare unless --mock")
	runCmd.Flags().BoolVar(&flagJSON, "json", false, "Emit JSON report to stdout")
	runCmd.Flags().BoolVar(&flagMock, "mock", false, "Dry-run / mock mode: no network; uses probe.mock")
	runCmd.Flags().StringVar(&flagProbes, "probes", "probes/default", "Directory of probe YAML files")
	runCmd.Flags().DurationVar(&flagTimeout, "timeout", 60*time.Second, "Per-probe timeout")
	runCmd.Flags().IntVar(&flagRepeat, "repeat", 1, "Run each probe N times (for flaky live APIs)")
	runCmd.Flags().Float64Var(&flagMinPass, "min-pass", 1.0, "Minimum pass rate (0-1) required to exit 0")
	runCmd.Flags().StringVar(&flagBaseline, "baseline", "", "Compare against baseline JSON; fail on regression")
	runCmd.Flags().StringVar(&flagWriteBaseline, "write-baseline", "", "Write current results as baseline JSON")
	runCmd.Flags().StringVar(&flagHTML, "html", "", "Also write HTML scorecard to this path (e.g. scorecard.html)")
	runCmd.Flags().StringVar(&flagLastPath, "last", report.DefaultLastPath, "Where to save the last JSON report")

	reportCmd.Flags().BoolVar(&flagReportLast, "last", true, "Use the last saved JSON report (.toolprobe/last-report.json)")
	reportCmd.Flags().StringVar(&flagReportLastPath, "from", "", "JSON report path (overrides --last default)")
	reportCmd.Flags().StringVar(&flagReportOut, "out", report.DefaultScorecardPath, "HTML scorecard output path")
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

	provName, err := provider.Canonical(flagProvider)
	if err != nil {
		return err
	}
	modelSet := cmd.Flags().Changed("model") || os.Getenv("TOOLPROBE_MODEL") != ""
	model, err := provider.ResolveModel(provName, flagModel, modelSet, flagMock)
	if err != nil {
		return err
	}
	apiKey := provider.ResolveAPIKey(provName, flagAPIKey, cmd.Flags().Changed("api-key"))
	accountID := provider.ResolveAccountID(flagAccountID, cmd.Flags().Changed("account-id"))

	var runner *probes.Runner
	if flagMock && provName == provider.OpenAI {
		c := client.New("", "")
		c.Mock = true
		runner = &probes.Runner{Client: c, Model: model}
	} else if flagMock {
		runner, err = probes.NewNativeMockRunner(provName, model)
		if err != nil {
			return err
		}
		model = runner.Model
	} else {
		baseURL := flagBaseURL
		if baseURL == "" {
			baseURL = provider.DefaultBaseURL(provName)
		}
		p, err := provider.New(provider.Config{
			Provider:  provName,
			BaseURL:   baseURL,
			APIKey:    apiKey,
			AccountID: accountID,
		})
		if err != nil {
			return err
		}
		runner = &probes.Runner{Provider: p, Model: model}
	}

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
		Provider:    provName,
		Model:       model,
		Mock:        flagMock,
		Repeat:      flagRepeat,
		MinPass:     flagMinPass,
		Summary:     sum,
		Results:     results,
	}

	if flagLastPath != "" {
		if err := report.SaveJSON(flagLastPath, rep); err != nil {
			return fmt.Errorf("save last report: %w", err)
		}
	}

	if flagHTML != "" {
		if err := report.SaveHTML(flagHTML, rep); err != nil {
			return fmt.Errorf("write html: %w", err)
		}
		if !flagJSON {
			fmt.Fprintf(os.Stderr, "wrote %s\n", flagHTML)
		}
	}

	if flagWriteBaseline != "" {
		b := baseline.FromReport(rep)
		if err := baseline.Save(flagWriteBaseline, b); err != nil {
			return fmt.Errorf("write baseline: %w", err)
		}
		if !flagJSON {
			fmt.Fprintf(os.Stderr, "wrote baseline %s\n", flagWriteBaseline)
		}
	}

	baselineFailed := false
	if flagBaseline != "" {
		base, err := baseline.Load(flagBaseline)
		if err != nil {
			return fmt.Errorf("load baseline: %w", err)
		}
		diff := baseline.Compare(*base, rep)
		if !diff.OK {
			baselineFailed = true
			for _, m := range diff.Messages {
				fmt.Fprintf(os.Stderr, "BASELINE: %s\n", m)
			}
		} else if !flagJSON {
			fmt.Fprintf(os.Stderr, "baseline OK (%s)\n", flagBaseline)
		}
	}

	if flagJSON {
		if err := report.WriteJSON(os.Stdout, rep); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(os.Stderr, "\n%d/%d passed (%.0f%%)  avg latency %.1fms  min-pass %.0f%%\n",
			sum.Passed, sum.Total, sum.PassRate*100, sum.AvgLatency, flagMinPass*100)
	}

	if sum.PassRate < flagMinPass || baselineFailed {
		os.Exit(1)
	}
	return nil
}

func runReport(cmd *cobra.Command, args []string) error {
	path := flagReportLastPath
	if path == "" {
		if flagReportLast {
			path = report.DefaultLastPath
		} else {
			return fmt.Errorf("provide --last or --from <report.json>")
		}
	}
	r, err := report.LoadJSON(path)
	if err != nil {
		return fmt.Errorf("load report %s: %w (run `toolprobe run` first)", path, err)
	}
	out := flagReportOut
	if out == "" {
		out = report.DefaultScorecardPath
	}
	if err := report.SaveHTML(out, *r); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s from %s\n", out, path)
	return nil
}
