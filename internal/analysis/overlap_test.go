package analysis

import (
	"testing"

	"github.com/thinkwright/agent-evals/internal/loader"
)

func TestSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want float64
		tol  float64
	}{
		{"identical strings", "hello world", "hello world", 1.0, 0.01},
		{"both empty", "", "", 1.0, 0.01},
		{"one empty", "hello", "", 0.0, 0.01},
		{"other empty", "", "hello", 0.0, 0.01},
		{"completely different", "abcde", "fghij", 0.0, 0.01},
		{"partial overlap", "abcdef", "abcxyz", 0.5, 0.1},
		{"substring", "abc", "abcdef", 0.67, 0.1},
		{"single char match", "a", "a", 1.0, 0.01},
		{"single char differ", "a", "b", 0.0, 0.01},
		{"realistic prompts similar",
			"you are a backend api developer focusing on rest apis and databases",
			"you are a backend service developer focusing on rest apis and data stores",
			0.8, 0.15},
		{"realistic prompts different",
			"you are a backend api developer focusing on rest apis and databases",
			"you are a legal advisor specializing in contract law and compliance",
			0.55, 0.15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := similarity(tt.a, tt.b)
			if got < tt.want-tt.tol || got > tt.want+tt.tol {
				t.Errorf("similarity(%q, %q) = %.3f, want %.3f ± %.2f", tt.a, tt.b, got, tt.want, tt.tol)
			}
		})
	}
}

func TestSimilaritySymmetric(t *testing.T) {
	pairs := [][2]string{
		{"hello world", "world hello"},
		{"abc", "abcdef"},
		{"testing framework", "framework testing"},
	}
	for _, p := range pairs {
		ab := similarity(p[0], p[1])
		ba := similarity(p[1], p[0])
		if ab != ba {
			t.Errorf("similarity is not symmetric: (%q,%q)=%.3f but (%q,%q)=%.3f",
				p[0], p[1], ab, p[1], p[0], ba)
		}
	}
}

func TestStrongDomains(t *testing.T) {
	scores := map[string]float64{
		"backend":   0.8,
		"frontend":  0.5,
		"databases": 0.2,
		"security":  0.31,
	}

	got := strongDomains(scores, 0.3)

	if !got["backend"] {
		t.Error("expected backend to be strong (0.8 > 0.3)")
	}
	if !got["frontend"] {
		t.Error("expected frontend to be strong (0.5 > 0.3)")
	}
	if got["databases"] {
		t.Error("expected databases to not be strong (0.2 <= 0.3)")
	}
	if !got["security"] {
		t.Error("expected security to be strong (0.31 > 0.3)")
	}
}

func TestStrongDomainsExactThreshold(t *testing.T) {
	scores := map[string]float64{"backend": 0.3}
	got := strongDomains(scores, 0.3)
	if got["backend"] {
		t.Error("score exactly at threshold should not be included (strictly greater)")
	}
}

func TestStrongDomainsEmpty(t *testing.T) {
	got := strongDomains(nil, 0.3)
	if len(got) != 0 {
		t.Errorf("expected empty result for nil input, got %d entries", len(got))
	}
}

func TestIntersection(t *testing.T) {
	a := map[string]bool{"backend": true, "frontend": true, "databases": true}
	b := map[string]bool{"frontend": true, "security": true, "databases": true}

	got := intersection(a, b)

	if len(got) != 2 {
		t.Fatalf("expected 2 shared domains, got %d", len(got))
	}
	if !got["frontend"] || !got["databases"] {
		t.Errorf("expected frontend and databases in intersection, got %v", got)
	}
}

func TestIntersectionDisjoint(t *testing.T) {
	a := map[string]bool{"backend": true}
	b := map[string]bool{"frontend": true}
	got := intersection(a, b)
	if len(got) != 0 {
		t.Errorf("expected empty intersection for disjoint sets, got %v", got)
	}
}

func TestUnion(t *testing.T) {
	a := map[string]bool{"backend": true, "frontend": true}
	b := map[string]bool{"frontend": true, "security": true}

	got := union(a, b)
	if len(got) != 3 {
		t.Fatalf("expected 3 domains in union, got %d", len(got))
	}
	for _, d := range []string{"backend", "frontend", "security"} {
		if !got[d] {
			t.Errorf("expected %s in union", d)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "hello" {
		t.Errorf("truncate to 5: got %q, want %q", got, "hello")
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate shorter string: got %q, want %q", got, "hi")
	}
	if got := truncate("", 5); got != "" {
		t.Errorf("truncate empty: got %q, want %q", got, "")
	}
}

func TestDetectConflicts(t *testing.T) {
	a := &loader.AgentDefinition{
		ID:           "agent_a",
		SystemPrompt: "Always use PostgreSQL for data storage. Prefer tabs for indentation.",
	}
	b := &loader.AgentDefinition{
		ID:           "agent_b",
		SystemPrompt: "Never use PostgreSQL for any project. Avoid tabs in code.",
	}

	conflicts := detectConflicts(a, b)
	if len(conflicts) == 0 {
		t.Fatal("expected conflicts between agents with opposing instructions")
	}

	// Should detect the PostgreSQL conflict
	found := false
	for _, c := range conflicts {
		if containsAll(c, "postgresql") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PostgreSQL conflict, got: %v", conflicts)
	}
}

func TestDetectConflictsNone(t *testing.T) {
	a := &loader.AgentDefinition{
		ID:           "agent_a",
		SystemPrompt: "You are a backend developer specializing in Go and REST APIs.",
	}
	b := &loader.AgentDefinition{
		ID:           "agent_b",
		SystemPrompt: "You are a frontend developer specializing in React and CSS.",
	}

	conflicts := detectConflicts(a, b)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts between non-overlapping agents, got: %v", conflicts)
	}
}

func TestDetectConflictsDeduplication(t *testing.T) {
	a := &loader.AgentDefinition{
		ID:           "agent_a",
		SystemPrompt: "Always use typescript. Must always use typescript. Use typescript for everything.",
	}
	b := &loader.AgentDefinition{
		ID:           "agent_b",
		SystemPrompt: "Never use typescript. Avoid typescript. Don't use typescript.",
	}

	conflicts := detectConflicts(a, b)
	// Even with multiple matches, deduplication should limit results
	seen := make(map[string]bool)
	for _, c := range conflicts {
		if seen[c] {
			t.Errorf("duplicate conflict detected: %s", c)
		}
		seen[c] = true
	}
}

func TestComputeOverlapClean(t *testing.T) {
	a := &loader.AgentDefinition{ID: "backend", SystemPrompt: "You handle backend APIs."}
	b := &loader.AgentDefinition{ID: "frontend", SystemPrompt: "You handle frontend UIs."}

	domainMap := map[string]map[string]float64{
		"backend":  {"backend": 0.8, "databases": 0.6},
		"frontend": {"frontend": 0.9, "css": 0.7},
	}

	result := computeOverlap(a, b, domainMap)

	if result.Verdict != "clean" {
		t.Errorf("expected clean verdict for non-overlapping agents, got %q", result.Verdict)
	}
	if result.OverlapScore != 0.0 {
		t.Errorf("expected 0 overlap for disjoint domains, got %.2f", result.OverlapScore)
	}
}

func TestComputeOverlapWarning(t *testing.T) {
	a := &loader.AgentDefinition{ID: "backend_a", SystemPrompt: "You are a backend developer."}
	b := &loader.AgentDefinition{ID: "backend_b", SystemPrompt: "You are a backend engineer."}

	// Both agents claim the same domains strongly
	domainMap := map[string]map[string]float64{
		"backend_a": {"backend": 0.9, "databases": 0.8, "api_design": 0.7},
		"backend_b": {"backend": 0.9, "databases": 0.8, "api_design": 0.7},
	}

	result := computeOverlap(a, b, domainMap)

	if result.Verdict != "warning" {
		t.Errorf("expected warning for high overlap, got %q", result.Verdict)
	}
	if result.OverlapScore < 0.5 {
		t.Errorf("expected overlap > 0.5 for identical domains, got %.2f", result.OverlapScore)
	}
}

func TestComputeOverlapConflict(t *testing.T) {
	a := &loader.AgentDefinition{
		ID:           "agent_a",
		SystemPrompt: "Always use PostgreSQL for data storage.",
	}
	b := &loader.AgentDefinition{
		ID:           "agent_b",
		SystemPrompt: "Never use PostgreSQL in any project.",
	}

	domainMap := map[string]map[string]float64{
		"agent_a": {"databases": 0.8},
		"agent_b": {"databases": 0.8},
	}

	result := computeOverlap(a, b, domainMap)

	if result.Verdict != "conflict" {
		t.Errorf("expected conflict verdict, got %q", result.Verdict)
	}
	if len(result.ConflictingInstructions) == 0 {
		t.Error("expected conflicting instructions to be populated")
	}
}

func TestComputeOverlapsAllPairs(t *testing.T) {
	agents := []loader.AgentDefinition{
		{ID: "a", SystemPrompt: "Agent A"},
		{ID: "b", SystemPrompt: "Agent B"},
		{ID: "c", SystemPrompt: "Agent C"},
	}
	domainMap := map[string]map[string]float64{
		"a": {"backend": 0.5},
		"b": {"frontend": 0.5},
		"c": {"databases": 0.5},
	}

	results := ComputeOverlaps(agents, domainMap)

	// 3 agents → 3 pairs (a-b, a-c, b-c)
	if len(results) != 3 {
		t.Errorf("expected 3 pairwise results for 3 agents, got %d", len(results))
	}
}

// helper
func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
