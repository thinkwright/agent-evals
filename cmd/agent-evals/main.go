package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/thinkwright/agent-evals/internal/analysis"
	"github.com/thinkwright/agent-evals/internal/config"
	"github.com/thinkwright/agent-evals/internal/loader"
	"github.com/thinkwright/agent-evals/internal/probes"
	"github.com/thinkwright/agent-evals/internal/provider"
	"github.com/thinkwright/agent-evals/internal/report"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "agent-evals",
		Short:   "Overlap analysis, boundary testing, and metacognitive scoring for LLM agents",
		Version: version,
	}

	// Shared flags
	var (
		flagCI      bool
		flagFormat  string
		flagConfig  string
		flagOutput  string
		flagNoPager bool
	)

	// ── check command ────────────────────────────────────────────
	checkCmd := &cobra.Command{
		Use:   "check <path>",
		Short: "Static analysis only (no API calls)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			applyCIDefaults(cmd, &flagFormat, &flagNoPager, flagCI)
			agentsPath := args[0]

			cfg, err := config.Load(flagConfig, agentsPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			agents, err := loader.LoadAgents(agentsPath)
			if err != nil {
				return fmt.Errorf("load agents: %w", err)
			}
			if len(agents) == 0 {
				return fmt.Errorf("no agent definitions found in %s", agentsPath)
			}

			fmt.Fprintf(os.Stderr, "Loaded %d agent(s) from %s\n", len(agents), agentsPath)

			staticReport := analysis.RunStaticAnalysis(agents, cfg)

			output := formatReport(staticReport, nil, flagFormat)
			if err := writeOutput(output, flagOutput, flagFormat, flagNoPager); err != nil {
				return err
			}

			if flagCI {
				return checkCIResult(staticReport, nil, cfg)
			}
			return nil
		},
	}
	checkCmd.Flags().BoolVar(&flagCI, "ci", false, "CI mode: JSON output, no pager, exit 1 on failure")
	checkCmd.Flags().StringVar(&flagFormat, "format", "terminal", "Output format: terminal, json, markdown")
	checkCmd.Flags().StringVar(&flagConfig, "config", "", "Path to agent-evals.yaml config")
	checkCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Write report to file")
	checkCmd.Flags().BoolVar(&flagNoPager, "no-pager", false, "Disable automatic paging")

	// ── test command ─────────────────────────────────────────────
	var (
		flagProvider       string
		flagModel          string
		flagBaseURL        string
		flagAPIKeyEnv      string
		flagProbeBudget    int
		flagStochasticRuns int
		flagConcurrency    int
		flagTranscript     string
	)

	testCmd := &cobra.Command{
		Use:   "test <path>",
		Short: "Static analysis + live probes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			applyCIDefaults(cmd, &flagFormat, &flagNoPager, flagCI)
			agentsPath := args[0]

			cfg, err := config.Load(flagConfig, agentsPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			agents, err := loader.LoadAgents(agentsPath)
			if err != nil {
				return fmt.Errorf("load agents: %w", err)
			}
			if len(agents) == 0 {
				return fmt.Errorf("no agent definitions found in %s", agentsPath)
			}

			fmt.Fprintf(os.Stderr, "Loaded %d agent(s) from %s\n", len(agents), agentsPath)

			// Static analysis
			staticReport := analysis.RunStaticAnalysis(agents, cfg)

			// Resolve provider config from flags and config file
			providerCfg := resolveProviderConfig(cfg, flagProvider, flagModel, flagBaseURL, flagAPIKeyEnv)

			client, err := provider.NewClient(providerCfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to initialize API client: %v\n", err)
				fmt.Fprintln(os.Stderr, "Set the appropriate API key env var (e.g. ANTHROPIC_API_KEY, OPENAI_API_KEY).")
				os.Exit(1)
			}

			// Generate probes
			probeQuestions := probes.GenerateProbes(agents, flagProbeBudget)
			stochastic := flagStochasticRuns
			totalCalls := len(probeQuestions) * (1 + stochastic)
			fmt.Fprintf(os.Stderr, "Generated %d probes (budget: %d)\n", len(probeQuestions), flagProbeBudget)
			fmt.Fprintf(os.Stderr, "Running %d API calls...\n", totalCalls)

			liveReport := probes.RunLiveProbes(
				context.Background(),
				agents,
				probeQuestions,
				client,
				probes.RunConfig{
					StochasticRuns: stochastic,
					BatchDelay:     300 * time.Millisecond,
					Concurrency:    flagConcurrency,
				},
				func(done, total int, agentID, probeID string) {
					fmt.Fprintf(os.Stderr, "  [%d/%d] %s / %s\n", done, total, agentID, probeID)
				},
			)

			output := formatReport(staticReport, liveReport, flagFormat)
			if err := writeOutput(output, flagOutput, flagFormat, flagNoPager); err != nil {
				return err
			}

			if flagTranscript != "" {
				transcript := report.FormatTranscript(liveReport)
				if err := os.WriteFile(flagTranscript, []byte(transcript), 0644); err != nil {
					return fmt.Errorf("write transcript: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Transcript written to %s\n", flagTranscript)
			}

			if flagCI {
				return checkCIResult(staticReport, liveReport, cfg)
			}
			return nil
		},
	}
	testCmd.Flags().BoolVar(&flagCI, "ci", false, "CI mode: JSON output, no pager, exit 1 on failure")
	testCmd.Flags().StringVar(&flagFormat, "format", "terminal", "Output format: terminal, json, markdown")
	testCmd.Flags().StringVar(&flagConfig, "config", "", "Path to agent-evals.yaml config")
	testCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Write report to file")
	testCmd.Flags().BoolVar(&flagNoPager, "no-pager", false, "Disable automatic paging")
	testCmd.Flags().StringVar(&flagProvider, "provider", "anthropic", "LLM provider: anthropic, openai, openai-compatible")
	testCmd.Flags().StringVar(&flagModel, "model", "", "Model to use for probes")
	testCmd.Flags().StringVar(&flagBaseURL, "base-url", "", "Base URL for openai-compatible provider")
	testCmd.Flags().StringVar(&flagAPIKeyEnv, "api-key-env", "", "Environment variable name for API key")
	testCmd.Flags().IntVar(&flagProbeBudget, "probe-budget", 500, "Max API calls for live probes")
	testCmd.Flags().IntVar(&flagStochasticRuns, "stochastic-runs", 5, "Stochastic runs per probe")
	testCmd.Flags().IntVar(&flagConcurrency, "concurrency", 3, "Max concurrent API calls")
	testCmd.Flags().StringVar(&flagTranscript, "transcript", "", "Write full probe Q&A transcript to file (markdown)")

	root.AddCommand(checkCmd, testCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func formatReport(static *analysis.StaticReport, live *probes.LiveProbeReport, format string) string {
	switch format {
	case "json":
		return report.FormatJSON(static, live)
	case "markdown":
		return report.FormatMarkdown(static, live)
	default:
		return report.FormatTerminal(static, live)
	}
}

func writeOutput(output, path, format string, noPager bool) error {
	// Write to file
	if path != "" {
		if err := os.WriteFile(path, []byte(output), 0644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Report written to %s\n", path)
		return nil
	}

	// Use pager for terminal format when stdout is a TTY
	if format == "terminal" && !noPager && isTerminal() {
		return outputWithPager(output)
	}

	fmt.Print(output)
	return nil
}

// isTerminal returns true if stdout is connected to a terminal.
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// outputWithPager pipes output through a pager (less -R by default).
func outputWithPager(output string) error {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}

	// Build args: for less, -R preserves ANSI colors,
	// -X leaves output on screen after quit
	var args []string
	if pager == "less" {
		args = []string{"-R", "-X"}
	}

	cmd := exec.Command(pager, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		// Fall back to direct output
		fmt.Print(output)
		return nil
	}

	if err := cmd.Start(); err != nil {
		// Pager not available, fall back to direct output
		fmt.Print(output)
		return nil
	}

	io.WriteString(stdin, output)
	stdin.Close()

	// Ignore pager exit errors (e.g. user quits with 'q')
	cmd.Wait()
	return nil
}

func checkCIResult(static *analysis.StaticReport, live *probes.LiveProbeReport, cfg map[string]any) error {
	thresholds := getMapFromConfig(cfg, "thresholds")
	minOverall := getFloatFromConfig(thresholds, "min_overall_score", 0.7)

	if static.HasFailures() || static.Overall < minOverall {
		return fmt.Errorf("check failed: overall score %.0f%% below threshold %.0f%%", static.Overall*100, minOverall*100)
	}

	if live != nil {
		minBoundary := getFloatFromConfig(thresholds, "min_boundary_score", 0.5)
		for agentID, results := range live.AgentResults {
			if results.ProbesRun > 0 && results.BoundaryScore < minBoundary {
				return fmt.Errorf("check failed: agent '%s' boundary score %.0f%% below threshold %.0f%%",
					agentID, results.BoundaryScore*100, minBoundary*100)
			}
		}
	}

	return nil
}

// applyCIDefaults sets machine-friendly defaults when --ci is used:
// JSON format and no pager, unless the user explicitly overrode them.
func applyCIDefaults(cmd *cobra.Command, format *string, noPager *bool, ci bool) {
	if !ci {
		return
	}
	if !cmd.Flags().Changed("format") {
		*format = "json"
	}
	*noPager = true
}

func resolveProviderConfig(cfg map[string]any, flagProvider, flagModel, flagBaseURL, flagAPIKeyEnv string) provider.Config {
	probesCfg := getMapFromConfig(cfg, "probes")

	p := provider.Config{
		Provider: flagProvider,
		Model:    flagModel,
		BaseURL:  flagBaseURL,
	}

	// Fill from config file if flags not set
	if p.Model == "" {
		if m, ok := probesCfg["model"].(string); ok {
			p.Model = m
		}
	}
	if p.Provider == "anthropic" {
		if prov, ok := probesCfg["provider"].(string); ok {
			p.Provider = prov
		}
	}
	if p.BaseURL == "" {
		if u, ok := probesCfg["base_url"].(string); ok {
			p.BaseURL = u
		}
	}
	if flagAPIKeyEnv != "" {
		p.APIKeyEnv = flagAPIKeyEnv
	} else if env, ok := probesCfg["api_key_env"].(string); ok {
		p.APIKeyEnv = env
	}

	return p
}

func getMapFromConfig(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	if mm, ok := v.(map[string]any); ok {
		return mm
	}
	return nil
}

func getFloatFromConfig(m map[string]any, key string, fallback float64) float64 {
	if m == nil {
		return fallback
	}
	v, ok := m[key]
	if !ok {
		return fallback
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	}
	return fallback
}
