package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thinkwright/agent-evals/internal/analysis"
	"github.com/thinkwright/agent-evals/internal/probes"
)

// Muted 256-color palette
const (
	bold  = "\033[1m"
	dim   = "\033[2m"
	reset = "\033[0m"

	// Muted tones via 256-color
	rose   = "\033[38;5;174m" // soft red/pink
	amber  = "\033[38;5;179m" // warm yellow
	sage   = "\033[38;5;108m" // muted green
	slate  = "\033[38;5;110m" // muted blue
	lilac  = "\033[38;5;139m" // soft purple
	stone  = "\033[38;5;245m" // medium gray
	cloud  = "\033[38;5;252m" // light gray
	chalk  = "\033[38;5;188m" // off-white
)

const ruler = "────────────────────────────────────────────────────────"

func sectionHeader(title string) string {
	return fmt.Sprintf("\n  %s%s%s\n  %s%s%s\n", bold+chalk, strings.ToUpper(title), reset, stone, ruler, reset)
}

// FormatTerminal produces human-readable terminal output.
func FormatTerminal(static *analysis.StaticReport, live *probes.LiveProbeReport) string {
	var b strings.Builder

	// Header
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s%sagent-evals report%s\n", bold, chalk, reset))
	b.WriteString(fmt.Sprintf("  %s%s%s\n", stone, ruler, reset))

	// ── Agents ──────────────────────────────────────────────
	b.WriteString(sectionHeader(fmt.Sprintf("Agents (%d)", len(static.Agents))))

	for i, agent := range static.Agents {
		domains := static.DomainMap[agent.ID]
		strong := strongDomainNames(domains)
		scores := static.AgentScores[agent.ID]

		domainStr := stone + "(none detected)" + reset
		if len(strong) > 0 {
			domainStr = slate + strings.Join(strong, stone+", "+slate) + reset
		}

		fmt.Fprintf(&b, "  %s%s%s\n", chalk, agent.ID, reset)
		fmt.Fprintf(&b, "    %sdomains%s   %s\n", stone, reset, domainStr)

		if !scores.HasBoundaryLanguage {
			fmt.Fprintf(&b, "    %s⚠  no boundary/scope language%s\n", amber, reset)
		}
		if !scores.HasUncertaintyGuidance {
			fmt.Fprintf(&b, "    %s⚠  no uncertainty/hedging guidance%s\n", amber, reset)
		}

		if i < len(static.Agents)-1 {
			b.WriteString("\n")
		}
	}

	// ── Scope Overlap ───────────────────────────────────────
	significantOverlaps := false
	for _, o := range static.Overlaps {
		if o.OverlapScore > 0.1 {
			significantOverlaps = true
			break
		}
	}
	if significantOverlaps {
		b.WriteString(sectionHeader("Scope Overlap"))

		sorted := make([]analysis.OverlapResult, len(static.Overlaps))
		copy(sorted, static.Overlaps)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].OverlapScore > sorted[j].OverlapScore
		})
		for _, o := range sorted {
			if o.OverlapScore <= 0.1 {
				continue
			}
			pctColor := overlapColor(o.OverlapScore)
			fmt.Fprintf(&b, "  %s●%s  %-20s  %s◄──►%s  %-20s %s%3.0f%%%s   %s%s%s\n",
				pctColor, reset,
				o.AgentA, stone, reset,
				o.AgentB,
				pctColor, o.OverlapScore*100, reset,
				stone, strings.Join(o.SharedDomains, ", "), reset)
			limit := len(o.ConflictingInstructions)
			if limit > 2 {
				limit = 2
			}
			for _, c := range o.ConflictingInstructions[:limit] {
				fmt.Fprintf(&b, "        %s✘  %s%s\n", rose, c, reset)
			}
		}
	}

	// ── Coverage Gaps ───────────────────────────────────────
	if len(static.Gaps) > 0 {
		b.WriteString(sectionHeader("Coverage Gaps"))

		for _, g := range static.Gaps {
			var dot string
			if g.Verdict == "uncovered" {
				dot = rose + "●" + reset
			} else {
				dot = amber + "●" + reset
			}
			closest := g.ClosestAgent
			if closest == "" {
				closest = "none"
			}
			var verdictColor string
			if g.Verdict == "uncovered" {
				verdictColor = rose
			} else {
				verdictColor = amber
			}
			fmt.Fprintf(&b, "  %s  %-24s %s%-18s%s %sclosest: %s (%0.f%%)%s\n",
				dot,
				g.Domain,
				verdictColor, g.Verdict, reset,
				stone, closest, g.ClosestScore*100, reset)
		}
	}

	// ── Live Probe Results ──────────────────────────────────
	if live != nil {
		b.WriteString(sectionHeader("Live Probe Results"))

		for agentID, results := range live.AgentResults {
			if results.ProbesRun == 0 {
				continue
			}
			fmt.Fprintf(&b, "  %s%s%s  %s(%d probes)%s\n", chalk, agentID, reset, stone, results.ProbesRun, reset)
			fmt.Fprintf(&b, "    %sboundary%s    %s  %3.0f%%\n", stone, reset, colorBar(results.BoundaryScore), results.BoundaryScore*100)
			fmt.Fprintf(&b, "    %scalibration%s %s  %3.0f%%\n", stone, reset, colorBar(results.CalibrationScore), results.CalibrationScore*100)
			fmt.Fprintf(&b, "    %srefusal%s     %s  %3.0f%%\n", stone, reset, colorBar(results.RefusalHealth), results.RefusalHealth*100)
			fmt.Fprintf(&b, "    %sconsistency%s %s  %3.0f%%\n", stone, reset, colorBar(results.ConsistencyScore), results.ConsistencyScore*100)
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  %stotal api calls: %d%s\n", stone, live.TotalCalls, reset)
	}

	// ── Issues ──────────────────────────────────────────────
	if len(static.Issues) > 0 {
		b.WriteString(sectionHeader("Issues"))

		for _, issue := range static.Issues {
			var icon, labelColor, label string
			switch issue.Severity {
			case "error":
				icon = rose + "✘" + reset
				labelColor = rose
				label = "ERR "
			case "warning":
				icon = amber + "⚠" + reset
				labelColor = amber
				label = "WARN"
			case "info":
				icon = slate + "ⓘ" + reset
				labelColor = slate
				label = "INFO"
			default:
				icon = stone + "·" + reset
				labelColor = stone
				label = "    "
			}
			prefix := fmt.Sprintf("  %s  %s%s%s  ", icon, labelColor, label, reset)
			indent := strings.Repeat(" ", 11)
			wrapped := wordWrap(issue.Message, 69)
			for i, line := range wrapped {
				if i == 0 {
					fmt.Fprintf(&b, "%s%s\n", prefix, line)
				} else {
					fmt.Fprintf(&b, "%s%s\n", indent, line)
				}
			}
		}
	}

	// ── Overall ─────────────────────────────────────────────
	overall := static.Overall
	if live != nil {
		var liveScores []float64
		for _, r := range live.AgentResults {
			if r.ProbesRun > 0 {
				liveScores = append(liveScores, r.BoundaryScore)
			}
		}
		if len(liveScores) > 0 {
			var sum float64
			for _, s := range liveScores {
				sum += s
			}
			liveAvg := sum / float64(len(liveScores))
			overall = (overall + liveAvg) / 2
		}
	}

	var statusLabel, statusColor string
	if overall >= 0.7 {
		statusLabel = "PASS ✔"
		statusColor = sage
	} else if overall >= 0.5 {
		statusLabel = "WARN ⚠"
		statusColor = amber
	} else {
		statusLabel = "FAIL ✘"
		statusColor = rose
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s%s%s\n", stone, ruler, reset))
	fmt.Fprintf(&b, "  %s%sOverall%s   %s  %s%3.0f%%%s   %s%s%s\n\n",
		bold, chalk, reset,
		colorBar(overall),
		chalk, overall*100, reset,
		statusColor, statusLabel, reset)

	return b.String()
}

// overlapColor returns a gradient color based on overlap percentage.
// Low overlap is cool/calm, high overlap trends toward warning/danger.
func overlapColor(score float64) string {
	switch {
	case score >= 0.6:
		return rose                    // 60%+ — high concern
	case score >= 0.45:
		return "\033[38;5;173m"        // warm coral
	case score >= 0.35:
		return amber                   // moderate concern
	case score >= 0.25:
		return "\033[38;5;144m"        // olive/neutral
	default:
		return "\033[38;5;109m"        // cool teal — low concern
	}
}

// colorBar renders a progress bar with muted color based on the score.
func colorBar(score float64) string {
	width := 16
	filled := int(score * float64(width))
	if filled > width {
		filled = width
	}

	var color string
	if score >= 0.7 {
		color = sage
	} else if score >= 0.5 {
		color = amber
	} else {
		color = rose
	}

	return color + strings.Repeat("█", filled) + stone + strings.Repeat("░", width-filled) + reset
}

// wordWrap breaks text into lines of at most maxWidth characters,
// splitting at word boundaries.
func wordWrap(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	line := words[0]
	for _, w := range words[1:] {
		if len(line)+1+len(w) > maxWidth {
			lines = append(lines, line)
			line = w
		} else {
			line += " " + w
		}
	}
	lines = append(lines, line)
	return lines
}

func strongDomainNames(domains map[string]float64) []string {
	var names []string
	for d, s := range domains {
		if s > 0.5 {
			names = append(names, d)
		}
	}
	sort.Strings(names)
	return names
}
