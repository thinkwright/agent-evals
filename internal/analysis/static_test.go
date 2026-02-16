package analysis

import (
	"testing"

	"github.com/thinkwright/agent-evals/internal/loader"
)

func TestRunStaticAnalysisEndToEnd(t *testing.T) {
	agents := []loader.AgentDefinition{
		{
			ID:           "backend_api",
			SystemPrompt: "You are a backend API developer. Build REST APIs with Go. Always use PostgreSQL for data storage. Do not answer questions outside backend development.",
		},
		{
			ID:           "frontend_react",
			SystemPrompt: "You are a frontend developer using React. Handle CSS, HTML, and browser DOM. Never use PostgreSQL directly from the frontend.",
		},
	}

	report := RunStaticAnalysis(agents, nil)

	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Should have domain maps for both agents
	if len(report.DomainMap) != 2 {
		t.Errorf("expected domain maps for 2 agents, got %d", len(report.DomainMap))
	}

	// backend_api should have backend domain
	if report.DomainMap["backend_api"]["backend"] == 0 {
		t.Error("expected backend domain for backend_api agent")
	}

	// frontend_react should have frontend domain
	if report.DomainMap["frontend_react"]["frontend"] == 0 {
		t.Error("expected frontend domain for frontend_react agent")
	}

	// Should detect PostgreSQL conflict
	hasConflict := false
	for _, issue := range report.Issues {
		if issue.Category == "conflict" {
			hasConflict = true
			break
		}
	}
	if !hasConflict {
		t.Error("expected conflict issue for opposing PostgreSQL instructions")
	}

	// backend_api has boundary language ("do not answer questions outside")
	if !report.AgentScores["backend_api"].HasBoundaryLanguage {
		t.Error("expected backend_api to have boundary language detected")
	}

	// frontend_react likely lacks explicit uncertainty guidance
	if report.AgentScores["frontend_react"].HasUncertaintyGuidance {
		t.Error("expected frontend_react to lack uncertainty guidance")
	}

	// Overall score should be < 1.0 due to issues
	if report.Overall >= 1.0 {
		t.Errorf("expected overall < 1.0 with issues present, got %.2f", report.Overall)
	}
}

func TestRunStaticAnalysisCleanAgents(t *testing.T) {
	agents := []loader.AgentDefinition{
		{
			ID:           "backend",
			SystemPrompt: "You are a backend developer. Do not answer frontend questions. When uncertain, hedge your response with caveats.",
		},
		{
			ID:           "frontend",
			SystemPrompt: "You handle frontend UI. Avoid answering backend questions. Express confidence levels when unsure.",
		},
	}

	report := RunStaticAnalysis(agents, nil)

	// Both have boundary language and uncertainty guidance, non-overlapping
	hasError := false
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			hasError = true
		}
	}
	if hasError {
		t.Error("expected no error-severity issues for clean non-overlapping agents")
	}
}

func TestRunStaticAnalysisSingleAgent(t *testing.T) {
	agents := []loader.AgentDefinition{
		{
			ID:           "solo",
			SystemPrompt: "You are a general purpose assistant.",
		},
	}

	report := RunStaticAnalysis(agents, nil)

	// Single agent → no overlaps
	if len(report.Overlaps) != 0 {
		t.Errorf("expected 0 overlaps for single agent, got %d", len(report.Overlaps))
	}
}

func TestRunStaticAnalysisCustomThresholds(t *testing.T) {
	agents := []loader.AgentDefinition{
		{
			ID:           "agent_a",
			SystemPrompt: "You handle backend REST APIs and PostgreSQL databases.",
		},
		{
			ID:           "agent_b",
			SystemPrompt: "You handle backend services and database queries.",
		},
	}

	// Strict threshold (default 0.3) should trigger overlap warning
	strict := RunStaticAnalysis(agents, nil)
	strictOverlaps := 0
	for _, issue := range strict.Issues {
		if issue.Category == "overlap" && issue.Severity == "warning" {
			strictOverlaps++
		}
	}

	// Very permissive threshold should suppress overlap warnings
	permissive := RunStaticAnalysis(agents, map[string]any{
		"thresholds": map[string]any{
			"max_overlap_score": 1.1, // impossible to exceed
		},
	})
	permissiveOverlaps := 0
	for _, issue := range permissive.Issues {
		if issue.Category == "overlap" && issue.Severity == "warning" {
			permissiveOverlaps++
		}
	}

	if permissiveOverlaps >= strictOverlaps && strictOverlaps > 0 {
		t.Errorf("permissive threshold should produce fewer overlap warnings: strict=%d, permissive=%d",
			strictOverlaps, permissiveOverlaps)
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0.0, "0%"},
		{1.0, "100%"},
		{0.5, "50%"},
		{0.333, "33%"},
		{0.667, "67%"},
		{0.999, "100%"},
		{0.001, "0%"},
		{0.505, "51%"},
	}

	for _, tt := range tests {
		got := formatPercent(tt.input)
		if got != tt.want {
			t.Errorf("formatPercent(%.3f) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHasFailures(t *testing.T) {
	report := &StaticReport{
		Issues: []Issue{
			{Severity: "warning", Category: "overlap"},
			{Severity: "info", Category: "boundary"},
		},
	}
	if report.HasFailures() {
		t.Error("expected no failures with only warning/info issues")
	}

	report.Issues = append(report.Issues, Issue{Severity: "error", Category: "conflict"})
	if !report.HasFailures() {
		t.Error("expected failures with an error-severity issue")
	}
}

func TestHasWarnings(t *testing.T) {
	report := &StaticReport{
		Issues: []Issue{
			{Severity: "info", Category: "boundary"},
		},
	}
	if report.HasWarnings() {
		t.Error("expected no warnings with only info issues")
	}

	report.Issues = append(report.Issues, Issue{Severity: "warning", Category: "overlap"})
	if !report.HasWarnings() {
		t.Error("expected warnings with a warning-severity issue")
	}
}

func TestOverallScoreCalculation(t *testing.T) {
	// No issues → 1.0
	agents := []loader.AgentDefinition{
		{ID: "a", SystemPrompt: "Backend dev. Do not answer outside scope. When uncertain, hedge."},
		{ID: "b", SystemPrompt: "Frontend dev. Avoid backend. Express confidence when unsure."},
	}
	report := RunStaticAnalysis(agents, nil)

	// Check that overall is between 0 and 1
	if report.Overall < 0 || report.Overall > 1.0 {
		t.Errorf("overall score should be in [0, 1], got %.2f", report.Overall)
	}
}

func TestDomainSummaryBuiltinOnly(t *testing.T) {
	agents := []loader.AgentDefinition{
		{ID: "a", SystemPrompt: "You handle backend APIs."},
	}
	report := RunStaticAnalysis(agents, nil)

	if report.DomainSummary != "18 built-in domains" {
		t.Errorf("expected '18 built-in domains', got %q", report.DomainSummary)
	}
}

func TestDomainSummaryMixed(t *testing.T) {
	agents := []loader.AgentDefinition{
		{ID: "a", SystemPrompt: "You handle payments via Stripe."},
	}
	config := map[string]any{
		"domains": []any{
			"backend",
			"frontend",
			map[string]any{
				"name":     "payments",
				"keywords": []any{"stripe", "plaid"},
			},
		},
	}
	report := RunStaticAnalysis(agents, config)

	if report.DomainSummary != "2 built-in + 1 custom domains" {
		t.Errorf("expected '2 built-in + 1 custom domains', got %q", report.DomainSummary)
	}
}
