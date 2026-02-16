package analysis

import (
	"fmt"

	"github.com/thinkwright/agent-evals/internal/loader"
)

// Issue represents a finding from static analysis.
type Issue struct {
	Severity string // "error" | "warning" | "info"
	Category string // "conflict" | "overlap" | "gap" | "boundary" | "uncertainty"
	Message  string
	Agents   []string
	Score    float64
}

// StaticReport is the complete result of static analysis.
type StaticReport struct {
	Agents        []loader.AgentDefinition
	DomainMap     map[string]map[string]float64
	DomainSummary string // e.g. "18 built-in domains" or "3 built-in + 2 custom domains"
	Overlaps      []OverlapResult
	Gaps          []GapResult
	AgentScores   map[string]AgentScore
	Issues        []Issue
	Overall       float64
}

// HasFailures returns true if any issue is an error.
func (r *StaticReport) HasFailures() bool {
	for _, i := range r.Issues {
		if i.Severity == "error" {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any issue is a warning.
func (r *StaticReport) HasWarnings() bool {
	for _, i := range r.Issues {
		if i.Severity == "warning" {
			return true
		}
	}
	return false
}

// RunStaticAnalysis runs all static checks on a set of agent definitions.
func RunStaticAnalysis(agents []loader.AgentDefinition, config map[string]any) *StaticReport {
	if config == nil {
		config = make(map[string]any)
	}
	thresholds := getMap(config, "thresholds")

	// Resolve domain definitions from config
	resolvedDomains := ResolveDomains(config)

	// Extract domains for each agent
	domainMap := make(map[string]map[string]float64)
	for i := range agents {
		domainMap[agents[i].ID] = ExtractDomains(&agents[i], resolvedDomains)
	}

	// Pairwise overlap
	overlaps := ComputeOverlaps(agents, domainMap)

	// Collect all known domains from resolved set and extraction results
	allDomains := make(map[string]bool)
	for d := range resolvedDomains {
		allDomains[d] = true
	}
	for _, scores := range domainMap {
		for d := range scores {
			allDomains[d] = true
		}
	}

	// Gap analysis
	gaps := FindGaps(allDomains, domainMap)

	// Per-agent scores
	agentScores := make(map[string]AgentScore)
	for i := range agents {
		agentScores[agents[i].ID] = ScoreAgent(&agents[i], domainMap, overlaps)
	}

	// Compile issues
	issues := compileIssues(overlaps, gaps, agentScores, thresholds)

	// Overall score
	var overall float64
	if len(issues) > 0 {
		var errorCount, warnCount int
		for _, i := range issues {
			switch i.Severity {
			case "error":
				errorCount++
			case "warning":
				warnCount++
			}
		}
		overall = 1.0 - float64(errorCount)*0.2 - float64(warnCount)*0.05
		if overall < 0 {
			overall = 0
		}
	} else {
		overall = 1.0
	}

	// Build domain source summary
	domainSummary := buildDomainSummary(resolvedDomains)

	return &StaticReport{
		Agents:        agents,
		DomainMap:     domainMap,
		DomainSummary: domainSummary,
		Overlaps:      overlaps,
		Gaps:          gaps,
		AgentScores:   agentScores,
		Issues:        issues,
		Overall:       overall,
	}
}

func compileIssues(overlaps []OverlapResult, gaps []GapResult, agentScores map[string]AgentScore, thresholds map[string]any) []Issue {
	maxOverlap := getFloat(thresholds, "max_overlap_score", 0.3)
	var issues []Issue

	// Overlap issues
	for _, o := range overlaps {
		if o.Verdict == "conflict" {
			msg := "Conflicting instructions between '" + o.AgentA + "' and '" + o.AgentB + "'"
			if len(o.ConflictingInstructions) > 0 {
				limit := len(o.ConflictingInstructions)
				if limit > 3 {
					limit = 3
				}
				msg += ": "
				for i, c := range o.ConflictingInstructions[:limit] {
					if i > 0 {
						msg += "; "
					}
					msg += c
				}
			}
			issues = append(issues, Issue{
				Severity: "error",
				Category: "conflict",
				Message:  msg,
				Agents:   []string{o.AgentA, o.AgentB},
				Score:    o.OverlapScore,
			})
		} else if o.OverlapScore > maxOverlap {
			issues = append(issues, Issue{
				Severity: "warning",
				Category: "overlap",
				Message:  formatOverlapMessage(o),
				Agents:   []string{o.AgentA, o.AgentB},
				Score:    o.OverlapScore,
			})
		}
	}

	// Gap issues
	for _, g := range gaps {
		if g.Verdict == "uncovered" {
			closest := g.ClosestAgent
			if closest == "" {
				closest = "none"
			}
			issues = append(issues, Issue{
				Severity: "warning",
				Category: "gap",
				Message:  "Domain '" + g.Domain + "' has no agent with strong coverage",
				Agents:   nil,
				Score:    g.ClosestScore,
			})
		}
	}

	// Agent quality issues
	for agentID, scores := range agentScores {
		if !scores.HasBoundaryLanguage {
			issues = append(issues, Issue{
				Severity: "info",
				Category: "boundary",
				Message:  "Agent '" + agentID + "' has no boundary/scope language in its definition — may confidently answer outside its domain",
				Agents:   []string{agentID},
				Score:    scores.BoundaryDefScore,
			})
		}
		if !scores.HasUncertaintyGuidance {
			issues = append(issues, Issue{
				Severity: "info",
				Category: "uncertainty",
				Message:  "Agent '" + agentID + "' has no uncertainty guidance — may not hedge when it should",
				Agents:   []string{agentID},
				Score:    scores.UncertaintyGuidScore,
			})
		}
	}

	return issues
}

func formatOverlapMessage(o OverlapResult) string {
	msg := "High scope overlap (" + formatPercent(o.OverlapScore) + ") between '" + o.AgentA + "' and '" + o.AgentB + "'"
	if len(o.SharedDomains) > 0 {
		msg += " on domains: "
		for i, d := range o.SharedDomains {
			if i > 0 {
				msg += ", "
			}
			msg += d
		}
	}
	return msg
}

func formatPercent(f float64) string {
	pct := int(f*100 + 0.5)
	return fmt.Sprintf("%d%%", pct)
}

// helpers

func getMap(m map[string]any, key string) map[string]any {
	v, ok := m[key]
	if !ok {
		return make(map[string]any)
	}
	if mm, ok := v.(map[string]any); ok {
		return mm
	}
	return make(map[string]any)
}

func getFloat(m map[string]any, key string, fallback float64) float64 {
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

func buildDomainSummary(resolved map[string][]string) string {
	builtinCount := 0
	customCount := 0
	for name := range resolved {
		if _, ok := BuiltinDomains[name]; ok {
			builtinCount++
		} else {
			customCount++
		}
	}
	if customCount == 0 {
		return fmt.Sprintf("%d built-in domains", builtinCount)
	}
	return fmt.Sprintf("%d built-in + %d custom domains", builtinCount, customCount)
}
