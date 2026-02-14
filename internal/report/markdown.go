package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thinkwright/agent-evals/internal/analysis"
	"github.com/thinkwright/agent-evals/internal/probes"
)

// FormatMarkdown produces markdown for PR comments.
func FormatMarkdown(static *analysis.StaticReport, live *probes.LiveProbeReport) string {
	var b strings.Builder

	overall := static.Overall
	status := "❌ Fail"
	if overall >= 0.7 {
		status = "✅ Pass"
	} else if overall >= 0.5 {
		status = "⚠️ Warning"
	}
	fmt.Fprintf(&b, "## agent-evals: %s (%.0f%%)\n\n", status, overall*100)

	// Agent summary table
	b.WriteString("### Agents\n\n")
	if live != nil {
		b.WriteString("| Agent | Domains | Boundary | Calibration | Refusal | Consistency |\n")
		b.WriteString("|-------|---------|----------|-------------|---------|-------------|\n")
	} else {
		b.WriteString("| Agent | Domains | Scope Clarity | Boundary Def | Uncertainty |\n")
		b.WriteString("|-------|---------|---------------|--------------|-------------|\n")
	}

	for _, agent := range static.Agents {
		domains := static.DomainMap[agent.ID]
		strong := strongDomainNames(domains)
		domainStr := "—"
		if len(strong) > 0 {
			limit := len(strong)
			if limit > 3 {
				limit = 3
			}
			domainStr = strings.Join(strong[:limit], ", ")
		}

		if live != nil {
			if lr, ok := live.AgentResults[agent.ID]; ok {
				fmt.Fprintf(&b, "| %s | %s | %.0f%% | %.0f%% | %.0f%% | %.0f%% |\n",
					agent.ID, domainStr,
					lr.BoundaryScore*100, lr.CalibrationScore*100,
					lr.RefusalHealth*100, lr.ConsistencyScore*100)
			}
		} else {
			scores := static.AgentScores[agent.ID]
			fmt.Fprintf(&b, "| %s | %s | %.0f%% | %.0f%% | %.0f%% |\n",
				agent.ID, domainStr,
				scores.ScopeClarityScore*100,
				scores.BoundaryDefScore*100,
				scores.UncertaintyGuidScore*100)
		}
	}
	b.WriteString("\n")

	// Overlaps
	var significantOverlaps []analysis.OverlapResult
	for _, o := range static.Overlaps {
		if o.OverlapScore > 0.1 {
			significantOverlaps = append(significantOverlaps, o)
		}
	}
	if len(significantOverlaps) > 0 {
		b.WriteString("### Overlaps\n\n")
		sort.Slice(significantOverlaps, func(i, j int) bool {
			return significantOverlaps[i].OverlapScore > significantOverlaps[j].OverlapScore
		})
		for _, o := range significantOverlaps {
			emoji := "🟡"
			if o.Verdict == "conflict" {
				emoji = "🔴"
			}
			fmt.Fprintf(&b, "- %s **%s** ↔ **%s**: %.0f%% (%s)\n",
				emoji, o.AgentA, o.AgentB,
				o.OverlapScore*100,
				strings.Join(o.SharedDomains, ", "))
		}
		b.WriteString("\n")
	}

	// Issues
	var errors, warnings []analysis.Issue
	for _, i := range static.Issues {
		switch i.Severity {
		case "error":
			errors = append(errors, i)
		case "warning":
			warnings = append(warnings, i)
		}
	}
	if len(errors) > 0 || len(warnings) > 0 {
		b.WriteString("### Issues\n\n")
		for _, issue := range append(errors, warnings...) {
			emoji := "⚠️"
			if issue.Severity == "error" {
				emoji = "❌"
			}
			fmt.Fprintf(&b, "- %s %s\n", emoji, issue.Message)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// FormatTranscript produces a detailed markdown transcript of all probe
// questions and raw LLM responses, useful for manual review.
func FormatTranscript(live *probes.LiveProbeReport) string {
	if live == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("# Probe Transcript\n\n")

	// Sort agent IDs for stable output
	var agentIDs []string
	for id := range live.AgentResults {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs)

	for _, agentID := range agentIDs {
		results := live.AgentResults[agentID]
		if len(results.Details) == 0 {
			continue
		}

		fmt.Fprintf(&b, "## %s\n\n", agentID)

		for i, detail := range results.Details {
			fmt.Fprintf(&b, "### Probe %d: %s (%s)\n\n", i+1, detail.ProbeID, detail.ProbeType)
			fmt.Fprintf(&b, "**Domain:** %s\n\n", detail.Domain)
			fmt.Fprintf(&b, "**Expected:** %s\n\n", detail.Expected)
			fmt.Fprintf(&b, "**Question:** %s\n\n", detail.Question)

			for _, resp := range detail.Responses {
				label := "deterministic"
				if resp.Temperature > 0 {
					label = fmt.Sprintf("T=%.1f, run %d", resp.Temperature, resp.Run)
				}

				if resp.Error != "" {
					fmt.Fprintf(&b, "#### Response (%s) - ERROR\n\n```\n%s\n```\n\n", label, resp.Error)
					continue
				}

				conf := "n/a"
				if resp.Confidence != nil {
					conf = fmt.Sprintf("%.0f", *resp.Confidence)
				}
				fmt.Fprintf(&b, "#### Response (%s)\n\n", label)
				fmt.Fprintf(&b, "- **Confidence:** %s\n", conf)
				fmt.Fprintf(&b, "- **Hedging:** %.2f\n", resp.HedgingScore)
				fmt.Fprintf(&b, "- **Refusal:** %v\n\n", resp.IsRefusal)
				fmt.Fprintf(&b, "```\n%s\n```\n\n", resp.Raw)
			}

			b.WriteString("---\n\n")
		}
	}

	fmt.Fprintf(&b, "*%d total API calls*\n", live.TotalCalls)
	return b.String()
}
