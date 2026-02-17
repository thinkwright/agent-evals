package report

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/thinkwright/agent-evals/internal/analysis"
	"github.com/thinkwright/agent-evals/internal/probes"
)

// FormatJSON produces machine-readable JSON for CI artifacts.
func FormatJSON(static *analysis.StaticReport, live *probes.LiveProbeReport) string {
	report := map[string]any{
		"timestamp":     time.Now().Format(time.RFC3339),
		"version":       "0.1.0",
		"overall_score": static.Overall,
		"pass":          static.Overall >= 0.7 && !static.HasFailures(),
	}

	// Agents
	var agents []map[string]any
	for _, agent := range static.Agents {
		entry := map[string]any{
			"id":      agent.ID,
			"name":    agent.Name,
			"source":  agent.SourcePath,
			"domains": static.DomainMap[agent.ID],
			"static_scores": map[string]any{
				"scope_clarity_score":        static.AgentScores[agent.ID].ScopeClarityScore,
				"boundary_definition_score":  static.AgentScores[agent.ID].BoundaryDefScore,
				"uncertainty_guidance_score": static.AgentScores[agent.ID].UncertaintyGuidScore,
				"has_boundary_language":      static.AgentScores[agent.ID].HasBoundaryLanguage,
				"has_uncertainty_guidance":   static.AgentScores[agent.ID].HasUncertaintyGuidance,
				"strong_domains":             static.AgentScores[agent.ID].StrongDomains,
				"weak_domains":               static.AgentScores[agent.ID].WeakDomains,
				"max_overlap_with_other":     static.AgentScores[agent.ID].MaxOverlapWithOther,
				"word_count":                 static.AgentScores[agent.ID].WordCount,
			},
		}

		if agent.ContentHash != "" {
			entry["content_hash"] = agent.ContentHash
		}
		if len(agent.AlsoFoundIn) > 0 {
			entry["also_found_in"] = agent.AlsoFoundIn
			entry["instance_count"] = 1 + len(agent.AlsoFoundIn)
		}

		if live != nil {
			if lr, ok := live.AgentResults[agent.ID]; ok {
				entry["live_scores"] = map[string]any{
					"boundary_score":    lr.BoundaryScore,
					"calibration_score": lr.CalibrationScore,
					"refusal_health":    lr.RefusalHealth,
					"consistency_score": lr.ConsistencyScore,
					"probes_run":        lr.ProbesRun,
				}
			}
		}

		agents = append(agents, entry)
	}
	report["agents"] = agents

	// Overlaps
	var overlaps []map[string]any
	for _, o := range static.Overlaps {
		if o.OverlapScore > 0.1 {
			overlaps = append(overlaps, map[string]any{
				"agents":         []string{o.AgentA, o.AgentB},
				"score":          round3(o.OverlapScore),
				"shared_domains": o.SharedDomains,
				"conflicts":      o.ConflictingInstructions,
				"verdict":        o.Verdict,
			})
		}
	}
	report["overlaps"] = overlaps

	// Gaps
	var gaps []map[string]any
	for _, g := range static.Gaps {
		gaps = append(gaps, map[string]any{
			"domain":        g.Domain,
			"verdict":       g.Verdict,
			"closest_agent": g.ClosestAgent,
			"closest_score": round3(g.ClosestScore),
		})
	}
	report["gaps"] = gaps

	// Issues
	var issues []map[string]any
	for _, i := range static.Issues {
		issues = append(issues, map[string]any{
			"severity": i.Severity,
			"category": i.Category,
			"message":  i.Message,
			"agents":   i.Agents,
			"score":    i.Score,
		})
	}
	report["issues"] = issues

	// Live summary
	if live != nil {
		probed := 0
		for _, r := range live.AgentResults {
			if r.ProbesRun > 0 {
				probed++
			}
		}
		report["live_summary"] = map[string]any{
			"total_api_calls": live.TotalCalls,
			"agents_probed":   probed,
		}
	}

	// Scan metadata (populated when recursive dedup was used)
	totalFiles := 0
	duplicatesCollapsed := 0
	for _, agent := range static.Agents {
		totalFiles += 1 + len(agent.AlsoFoundIn)
		duplicatesCollapsed += len(agent.AlsoFoundIn)
	}
	if duplicatesCollapsed > 0 {
		report["scan_metadata"] = map[string]any{
			"total_files_scanned":  totalFiles,
			"unique_agents":        len(static.Agents),
			"duplicates_collapsed": duplicatesCollapsed,
			"dedup_method":         "sha256-system-prompt",
		}
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal report: %s"}`, err)
	}
	return string(data)
}

func round3(f float64) float64 {
	return float64(int(f*1000+0.5)) / 1000
}
