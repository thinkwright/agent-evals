package analysis

import (
	"regexp"
	"strings"

	"github.com/thinkwright/agent-evals/internal/loader"
)

// AgentScore holds summary scores for a single agent.
type AgentScore struct {
	StrongDomains          []string
	WeakDomains            []string
	MaxOverlapWithOther    float64
	HasBoundaryLanguage    bool
	HasUncertaintyGuidance bool
	ScopeClarityScore      float64
	BoundaryDefScore       float64
	UncertaintyGuidScore   float64
	WordCount              int
}

var boundaryRe = regexp.MustCompile(`(?i)(don't|do not|avoid|outside|beyond|limit|scope|boundary|refer to)`)
var uncertaintyRe = regexp.MustCompile(`(?i)(uncertain|unsure|don't know|not sure|hedge|caveat|confidence)`)

// ScoreAgent computes summary scores for a single agent.
func ScoreAgent(agent *loader.AgentDefinition, domainMap map[string]map[string]float64, overlaps []OverlapResult) AgentScore {
	domains := domainMap[agent.ID]

	var strong, weak []string
	for d, s := range domains {
		if s > 0.5 {
			strong = append(strong, d)
		} else if s > 0.2 {
			weak = append(weak, d)
		}
	}

	var maxOverlap float64
	for _, o := range overlaps {
		if o.AgentA == agent.ID || o.AgentB == agent.ID {
			if o.OverlapScore > maxOverlap {
				maxOverlap = o.OverlapScore
			}
		}
	}

	prompt := strings.ToLower(agent.SystemPrompt)
	hasBoundary := boundaryRe.MatchString(prompt)
	hasUncertainty := uncertaintyRe.MatchString(prompt)

	var scopeScore float64
	if len(strong) > 0 {
		scopeScore = float64(len(strong)) / 3.0
		if scopeScore > 1.0 {
			scopeScore = 1.0
		}
	} else {
		scopeScore = 0.2
	}

	var boundaryScore float64
	if hasBoundary {
		boundaryScore = 0.7
	} else {
		boundaryScore = 0.3
	}

	var uncertaintyScore float64
	if hasUncertainty {
		uncertaintyScore = 0.8
	} else {
		uncertaintyScore = 0.3
	}

	return AgentScore{
		StrongDomains:          strong,
		WeakDomains:            weak,
		MaxOverlapWithOther:    maxOverlap,
		HasBoundaryLanguage:    hasBoundary,
		HasUncertaintyGuidance: hasUncertainty,
		ScopeClarityScore:      scopeScore,
		BoundaryDefScore:       boundaryScore,
		UncertaintyGuidScore:   uncertaintyScore,
		WordCount:              agent.WordCount(),
	}
}
