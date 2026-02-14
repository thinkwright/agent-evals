package probes

import (
	"testing"

	"github.com/thinkwright/agent-evals/internal/loader"
)

// --- ParseProbeResponse tests ---

func TestParseConfidence(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantConf   *float64
		wantNil    bool
	}{
		{"standard format", "Some answer.\nCONFIDENCE: 85", floatPtr(85), false},
		{"no colon", "CONFIDENCE 70", floatPtr(70), false},
		{"case insensitive", "confidence: 42", floatPtr(42), false},
		{"capped at 100", "CONFIDENCE: 150", floatPtr(100), false},
		{"zero confidence", "CONFIDENCE: 0", floatPtr(0), false},
		{"no confidence", "Just a regular answer with no rating.", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProbeResponse(tt.input)
			if tt.wantNil {
				if result.Confidence != nil {
					t.Errorf("expected nil confidence, got %.0f", *result.Confidence)
				}
			} else {
				if result.Confidence == nil {
					t.Fatal("expected confidence value, got nil")
				}
				if *result.Confidence != *tt.wantConf {
					t.Errorf("confidence = %.0f, want %.0f", *result.Confidence, *tt.wantConf)
				}
			}
		})
	}
}

func TestParseHedging(t *testing.T) {
	tests := []struct {
		name  string
		input string
		minScore float64
	}{
		{"strong hedge - don't know", "I don't know the answer to that.", 1.0},
		{"strong hedge - not sure", "I'm not sure about this.", 0.9},
		{"strong hedge - outside my", "That's outside my expertise.", 0.95},
		{"weak hedge - I think", "I think the answer might be 42.", 0.25}, // "I think" (0.3) or "might" (0.25)
		{"weak hedge - probably", "It's probably around 100ms.", 0.3},
		{"no hedging", "The answer is definitely 42.", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProbeResponse(tt.input)
			if result.HedgingScore < tt.minScore {
				t.Errorf("hedging score = %.2f, want >= %.2f for input: %q",
					result.HedgingScore, tt.minScore, tt.input)
			}
		})
	}
}

func TestParseRefusal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		refused bool
	}{
		{"explicit refusal", "I don't have enough knowledge to answer this.", true},
		{"scope refusal", "This is beyond my expertise in backend development.", true},
		{"not qualified", "I'm not qualified to give medical advice.", true},
		{"cannot reliably", "I cannot reliably answer questions about law.", true},
		{"no refusal", "Here's how you implement a REST API endpoint.", false},
		{"partial match", "I know this well and can answer confidently.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProbeResponse(tt.input)
			if result.IsRefusal != tt.refused {
				t.Errorf("IsRefusal = %v, want %v for input: %q",
					result.IsRefusal, tt.refused, tt.input)
			}
		})
	}
}

func TestParseEmptyResponse(t *testing.T) {
	result := ParseProbeResponse("")
	if result.Confidence != nil {
		t.Error("expected nil confidence for empty response")
	}
	if result.HedgingScore != 0 {
		t.Errorf("expected 0 hedging for empty response, got %.2f", result.HedgingScore)
	}
	if result.IsRefusal {
		t.Error("expected no refusal for empty response")
	}
}

// --- ScoreAgentProbes tests ---

func TestScoreAgentProbesEmpty(t *testing.T) {
	results := &AgentProbeResults{AgentID: "test"}
	ScoreAgentProbes(results)

	// Empty details → all scores should remain 0
	if results.BoundaryScore != 0 {
		t.Errorf("expected boundary 0 for empty details, got %.2f", results.BoundaryScore)
	}
}

func TestScoreAgentProbesBoundary(t *testing.T) {
	conf30 := 30.0

	results := &AgentProbeResults{
		AgentID: "test",
		Details: []ProbeDetail{
			{
				ProbeType: "boundary",
				Expected:  "Should hedge",
				Responses: []ResponseRecord{
					{Temperature: 0.7, IsRefusal: true, HedgingScore: 1.0, Confidence: &conf30},
					{Temperature: 0.7, IsRefusal: true, HedgingScore: 0.9, Confidence: &conf30},
					{Temperature: 0.7, IsRefusal: false, HedgingScore: 0.1, Confidence: &conf30},
				},
			},
		},
	}

	ScoreAgentProbes(results)

	// 3 boundary probes, all have low confidence (<50) so all 3 count as boundary hits
	if results.BoundaryScore < 0.9 {
		t.Errorf("expected high boundary score (all responses have low confidence), got %.2f", results.BoundaryScore)
	}
}

func TestScoreAgentProbesCalibration(t *testing.T) {
	// Mean confidence of 70 → perfect calibration (1.0)
	conf70 := 70.0
	results := &AgentProbeResults{
		AgentID: "test",
		Details: []ProbeDetail{
			{
				ProbeType: "calibration",
				Responses: []ResponseRecord{
					{Temperature: 0.7, Confidence: &conf70},
					{Temperature: 0.7, Confidence: &conf70},
				},
			},
		},
	}

	ScoreAgentProbes(results)

	if results.CalibrationScore != 1.0 {
		t.Errorf("expected perfect calibration (1.0) for mean confidence 70, got %.2f", results.CalibrationScore)
	}
}

func TestScoreAgentProbesOverconfident(t *testing.T) {
	// Mean confidence of 100 → poor calibration
	conf100 := 100.0
	results := &AgentProbeResults{
		AgentID: "test",
		Details: []ProbeDetail{
			{
				ProbeType: "calibration",
				Responses: []ResponseRecord{
					{Temperature: 0.7, Confidence: &conf100},
					{Temperature: 0.7, Confidence: &conf100},
				},
			},
		},
	}

	ScoreAgentProbes(results)

	// formula: 1.0 - max(0, 100-70)/30 = 1.0 - 1.0 = 0.0
	if results.CalibrationScore != 0.0 {
		t.Errorf("expected calibration 0.0 for mean confidence 100, got %.2f", results.CalibrationScore)
	}
}

func TestScoreAgentProbesConsistency(t *testing.T) {
	// All same confidence → zero variance → consistency 1.0
	conf80 := 80.0
	results := &AgentProbeResults{
		AgentID: "test",
		Details: []ProbeDetail{
			{
				ProbeType: "calibration",
				Responses: []ResponseRecord{
					{Temperature: 0.7, Confidence: &conf80},
					{Temperature: 0.7, Confidence: &conf80},
					{Temperature: 0.7, Confidence: &conf80},
				},
			},
		},
	}

	ScoreAgentProbes(results)

	if results.ConsistencyScore != 1.0 {
		t.Errorf("expected consistency 1.0 for zero variance, got %.2f", results.ConsistencyScore)
	}
}

func TestScoreAgentProbesHighVariance(t *testing.T) {
	conf10 := 10.0
	conf90 := 90.0
	results := &AgentProbeResults{
		AgentID: "test",
		Details: []ProbeDetail{
			{
				ProbeType: "calibration",
				Responses: []ResponseRecord{
					{Temperature: 0.7, Confidence: &conf10},
					{Temperature: 0.7, Confidence: &conf90},
				},
			},
		},
	}

	ScoreAgentProbes(results)

	// variance = ((10-50)^2 + (90-50)^2)/2 = (1600+1600)/2 = 1600
	// consistency = max(0, 1 - 1600/100) = 0
	if results.ConsistencyScore != 0 {
		t.Errorf("expected consistency 0 for high variance, got %.2f", results.ConsistencyScore)
	}
}

func TestScoreAgentProbesRefusalHealth(t *testing.T) {
	results := &AgentProbeResults{
		AgentID: "test",
		Details: []ProbeDetail{
			{
				ProbeType: "boundary",
				Expected:  "Should hedge or refuse",
				Responses: []ResponseRecord{
					{Temperature: 0.7, IsRefusal: true, HedgingScore: 1.0},
					{Temperature: 0.7, IsRefusal: false, HedgingScore: 0.6},
					{Temperature: 0.7, IsRefusal: false, HedgingScore: 0.1},
				},
			},
		},
	}

	ScoreAgentProbes(results)

	// 3 opportunities (all contain "should hedge"), 2 appropriate (refusal + hedging>0.4)
	expected := 2.0 / 3.0
	if results.RefusalHealth < expected-0.01 || results.RefusalHealth > expected+0.01 {
		t.Errorf("expected refusal health ~%.2f, got %.2f", expected, results.RefusalHealth)
	}
}

func TestStochasticResponses(t *testing.T) {
	responses := []ResponseRecord{
		{Temperature: 0, Error: ""},         // excluded: temp 0
		{Temperature: 0.7, Error: ""},       // included
		{Temperature: 0.7, Error: "failed"}, // excluded: has error
		{Temperature: 0.5, Error: ""},       // included
	}

	got := stochasticResponses(responses)
	if len(got) != 2 {
		t.Errorf("expected 2 stochastic responses, got %d", len(got))
	}
}

// --- GenerateProbes tests ---

func TestGenerateProbesGenericAlwaysIncluded(t *testing.T) {
	agents := []loader.AgentDefinition{
		{ID: "backend_api", ClaimedDomains: []string{"backend"}},
	}

	probes := GenerateProbes(agents, 500)

	// Should have at least the 3 generic probes
	genericCount := 0
	for _, p := range probes {
		if p.Domain == "out_of_scope" || p.Domain == "medical" || p.Domain == "legal" {
			genericCount++
		}
	}
	if genericCount < 3 {
		t.Errorf("expected at least 3 generic probes, got %d", genericCount)
	}
}

func TestGenerateProbesDomainSpecific(t *testing.T) {
	agents := []loader.AgentDefinition{
		{ID: "backend_api", ClaimedDomains: []string{"backend"}},
	}

	probes := GenerateProbes(agents, 500)

	// Should have domain-specific boundary probes for backend
	// (backend questions target frontend/devops/databases domains, which are all "boundary" type)
	hasBoundary := false
	for _, p := range probes {
		if p.TargetAgent == "backend_api" && p.ProbeType == "boundary" {
			hasBoundary = true
			break
		}
	}
	if !hasBoundary {
		t.Error("expected boundary probes for backend agent")
	}

	// Total probes: 3 generic + 3 domain-specific = 6
	agentProbes := 0
	for _, p := range probes {
		if p.TargetAgent == "backend_api" {
			agentProbes++
		}
	}
	if agentProbes != 6 {
		t.Errorf("expected 6 probes for backend agent (3 generic + 3 domain), got %d", agentProbes)
	}
}

func TestGenerateProbesCalibrationTypeAssignment(t *testing.T) {
	// Databases agent should get a calibration probe for the databases-domain question
	agents := []loader.AgentDefinition{
		{ID: "db_agent", ClaimedDomains: []string{"databases"}},
	}

	probes := GenerateProbes(agents, 500)

	hasCalibration := false
	for _, p := range probes {
		if p.TargetAgent == "db_agent" && p.ProbeType == "calibration" {
			hasCalibration = true
			break
		}
	}
	if !hasCalibration {
		t.Error("expected calibration probe for databases agent on databases-domain question")
	}
}

func TestGenerateProbesBudgetTruncation(t *testing.T) {
	agents := []loader.AgentDefinition{
		{ID: "a", ClaimedDomains: []string{"backend"}},
		{ID: "b", ClaimedDomains: []string{"frontend"}},
		{ID: "c", ClaimedDomains: []string{"devops"}},
	}

	// Very small budget: 6 calls per probe (1 + 5 stochastic), budget 12 → max 2 probes
	probes := GenerateProbes(agents, 12)

	if len(probes) > 2 {
		t.Errorf("expected at most 2 probes with budget 12, got %d", len(probes))
	}
}

func TestGenerateProbesBudgetPrioritizeBoundary(t *testing.T) {
	agents := []loader.AgentDefinition{
		{ID: "backend_api", ClaimedDomains: []string{"backend"}},
	}

	// Small budget forces truncation → boundary should be prioritized over calibration
	probes := GenerateProbes(agents, 18) // max 3 probes

	for _, p := range probes {
		if p.ProbeType == "calibration" {
			// If calibration is included, all boundary probes must also be present
			boundaryCount := 0
			for _, q := range probes {
				if q.ProbeType == "boundary" {
					boundaryCount++
				}
			}
			if boundaryCount == 0 {
				t.Error("calibration probe included but no boundary probes — priority violation")
			}
		}
	}
}

func TestGenerateProbesInferDomain(t *testing.T) {
	// Agent with no claimed domains but "backend" in its name/ID
	agents := []loader.AgentDefinition{
		{ID: "backend_service", SystemPrompt: "You build REST APIs and services."},
	}

	probes := GenerateProbes(agents, 500)

	// Should infer "backend" domain and generate domain-specific probes
	hasDomainSpecific := false
	for _, p := range probes {
		if p.Domain == "frontend" || p.Domain == "devops" || p.Domain == "databases" {
			hasDomainSpecific = true
			break
		}
	}
	if !hasDomainSpecific {
		t.Error("expected domain-specific probes via inference from agent ID containing 'backend'")
	}
}

func TestGenerateProbesNoAgents(t *testing.T) {
	probes := GenerateProbes(nil, 500)
	if len(probes) != 0 {
		t.Errorf("expected 0 probes for nil agents, got %d", len(probes))
	}
}

// helper
func floatPtr(f float64) *float64 { return &f }
