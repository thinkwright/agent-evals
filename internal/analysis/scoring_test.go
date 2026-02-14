package analysis

import (
	"testing"

	"github.com/thinkwright/agent-evals/internal/loader"
)

func TestScoreAgentBoundaryDetection(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		hasBoundary bool
	}{
		{"explicit boundary language", "Do not answer questions outside your scope.", true},
		{"avoid keyword", "Avoid providing legal advice.", true},
		{"refer to keyword", "Refer to a specialist for medical questions.", true},
		{"limit keyword", "Limit your responses to backend topics.", true},
		{"no boundary language", "You are a helpful coding assistant.", false},
		{"case insensitive", "BEYOND your expertise, refer to others.", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &loader.AgentDefinition{ID: "test", SystemPrompt: tt.prompt}
			domainMap := map[string]map[string]float64{"test": {"backend": 0.5}}
			score := ScoreAgent(agent, domainMap, nil)

			if score.HasBoundaryLanguage != tt.hasBoundary {
				t.Errorf("HasBoundaryLanguage = %v, want %v for prompt: %q",
					score.HasBoundaryLanguage, tt.hasBoundary, tt.prompt)
			}
		})
	}
}

func TestScoreAgentUncertaintyDetection(t *testing.T) {
	tests := []struct {
		name           string
		prompt         string
		hasUncertainty bool
	}{
		{"hedge keyword", "When unsure, hedge your response.", true},
		{"confidence keyword", "Express your confidence level.", true},
		{"caveat keyword", "Add caveats when appropriate.", true},
		{"no uncertainty", "You are a backend developer.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &loader.AgentDefinition{ID: "test", SystemPrompt: tt.prompt}
			domainMap := map[string]map[string]float64{"test": {}}
			score := ScoreAgent(agent, domainMap, nil)

			if score.HasUncertaintyGuidance != tt.hasUncertainty {
				t.Errorf("HasUncertaintyGuidance = %v, want %v for prompt: %q",
					score.HasUncertaintyGuidance, tt.hasUncertainty, tt.prompt)
			}
		})
	}
}

func TestScoreAgentScopeClarity(t *testing.T) {
	agent := &loader.AgentDefinition{ID: "test", SystemPrompt: "backend developer"}
	domainMap := map[string]map[string]float64{
		"test": {
			"backend":   0.9,
			"databases": 0.8,
			"frontend":  0.2,
		},
	}
	score := ScoreAgent(agent, domainMap, nil)

	// 2 strong domains (>0.5): backend, databases → scopeScore = 2/3 ≈ 0.67
	if score.ScopeClarityScore < 0.6 || score.ScopeClarityScore > 0.7 {
		t.Errorf("expected scope clarity ~0.67 for 2 strong domains, got %.2f", score.ScopeClarityScore)
	}
	if len(score.StrongDomains) != 2 {
		t.Errorf("expected 2 strong domains, got %d", len(score.StrongDomains))
	}
	if len(score.WeakDomains) != 0 {
		// frontend at 0.2 is not > 0.2, so it shouldn't be weak either
		t.Errorf("expected 0 weak domains (0.2 is not > 0.2), got %d", len(score.WeakDomains))
	}
}

func TestScoreAgentNoStrongDomains(t *testing.T) {
	agent := &loader.AgentDefinition{ID: "test", SystemPrompt: "generic assistant"}
	domainMap := map[string]map[string]float64{
		"test": {"backend": 0.1},
	}
	score := ScoreAgent(agent, domainMap, nil)

	if score.ScopeClarityScore != 0.2 {
		t.Errorf("expected default scope clarity 0.2 with no strong domains, got %.2f", score.ScopeClarityScore)
	}
}

func TestScoreAgentScopeClarityCapped(t *testing.T) {
	agent := &loader.AgentDefinition{ID: "test", SystemPrompt: "expert in everything"}
	domainMap := map[string]map[string]float64{
		"test": {
			"backend":   0.9,
			"databases": 0.8,
			"security":  0.7,
			"frontend":  0.6,
			"testing":   0.8,
		},
	}
	score := ScoreAgent(agent, domainMap, nil)

	// 5 strong domains → 5/3 = 1.67, capped at 1.0
	if score.ScopeClarityScore != 1.0 {
		t.Errorf("expected scope clarity capped at 1.0, got %.2f", score.ScopeClarityScore)
	}
}

func TestScoreAgentMaxOverlap(t *testing.T) {
	agent := &loader.AgentDefinition{ID: "agent_a", SystemPrompt: "backend developer"}
	domainMap := map[string]map[string]float64{"agent_a": {"backend": 0.5}}

	overlaps := []OverlapResult{
		{AgentA: "agent_a", AgentB: "agent_b", OverlapScore: 0.3},
		{AgentA: "agent_c", AgentB: "agent_a", OverlapScore: 0.7},
		{AgentA: "agent_d", AgentB: "agent_e", OverlapScore: 0.9}, // not involving agent_a
	}

	score := ScoreAgent(agent, domainMap, overlaps)

	if score.MaxOverlapWithOther != 0.7 {
		t.Errorf("expected max overlap 0.7 (from agent_c pair), got %.2f", score.MaxOverlapWithOther)
	}
}

func TestScoreAgentBoundaryScoreValues(t *testing.T) {
	withBoundary := &loader.AgentDefinition{ID: "a", SystemPrompt: "Do not answer outside your scope."}
	noBoundary := &loader.AgentDefinition{ID: "b", SystemPrompt: "You are a coding assistant."}
	dm := map[string]map[string]float64{"a": {}, "b": {}}

	scoreA := ScoreAgent(withBoundary, dm, nil)
	scoreB := ScoreAgent(noBoundary, dm, nil)

	if scoreA.BoundaryDefScore != 0.7 {
		t.Errorf("expected boundary score 0.7 with boundary language, got %.2f", scoreA.BoundaryDefScore)
	}
	if scoreB.BoundaryDefScore != 0.3 {
		t.Errorf("expected boundary score 0.3 without boundary language, got %.2f", scoreB.BoundaryDefScore)
	}
}
