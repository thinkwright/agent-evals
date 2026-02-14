package analysis

import (
	"testing"
)

func TestFindGapsUncovered(t *testing.T) {
	allDomains := map[string]bool{
		"backend":  true,
		"security": true,
		"testing":  true,
	}
	domainMap := map[string]map[string]float64{
		"agent_a": {"backend": 0.9, "security": 0.1, "testing": 0.0},
	}

	gaps := FindGaps(allDomains, domainMap)

	// security (0.1 < 0.2) → uncovered, testing (0.0 < 0.2) → uncovered
	if len(gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d: %+v", len(gaps), gaps)
	}

	verdicts := make(map[string]string)
	for _, g := range gaps {
		verdicts[g.Domain] = g.Verdict
	}

	if verdicts["security"] != "uncovered" {
		t.Errorf("expected security to be uncovered (score 0.1), got %q", verdicts["security"])
	}
	if verdicts["testing"] != "uncovered" {
		t.Errorf("expected testing to be uncovered (score 0.0), got %q", verdicts["testing"])
	}
}

func TestFindGapsWeaklyCovered(t *testing.T) {
	allDomains := map[string]bool{"security": true}
	domainMap := map[string]map[string]float64{
		"agent_a": {"security": 0.35},
	}

	gaps := FindGaps(allDomains, domainMap)

	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].Verdict != "weakly_covered" {
		t.Errorf("expected weakly_covered for score 0.35, got %q", gaps[0].Verdict)
	}
	if gaps[0].ClosestAgent != "agent_a" {
		t.Errorf("expected closest agent to be agent_a, got %q", gaps[0].ClosestAgent)
	}
}

func TestFindGapsWellCovered(t *testing.T) {
	allDomains := map[string]bool{"backend": true}
	domainMap := map[string]map[string]float64{
		"agent_a": {"backend": 0.8},
	}

	gaps := FindGaps(allDomains, domainMap)

	if len(gaps) != 0 {
		t.Errorf("expected no gaps for well-covered domain (score 0.8), got %+v", gaps)
	}
}

func TestFindGapsMultipleAgentsBestWins(t *testing.T) {
	allDomains := map[string]bool{"security": true}
	domainMap := map[string]map[string]float64{
		"agent_a": {"security": 0.1},
		"agent_b": {"security": 0.4},
		"agent_c": {"security": 0.15},
	}

	gaps := FindGaps(allDomains, domainMap)

	// Best score is 0.4 (agent_b), which is weakly_covered (0.2 <= 0.4 < 0.5)
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].Verdict != "weakly_covered" {
		t.Errorf("expected weakly_covered, got %q", gaps[0].Verdict)
	}
	if gaps[0].ClosestAgent != "agent_b" {
		t.Errorf("expected closest agent to be agent_b (best score), got %q", gaps[0].ClosestAgent)
	}
}

func TestFindGapsNoAgents(t *testing.T) {
	allDomains := map[string]bool{"backend": true, "security": true}
	domainMap := map[string]map[string]float64{}

	gaps := FindGaps(allDomains, domainMap)

	// All domains should be uncovered
	if len(gaps) != 2 {
		t.Fatalf("expected 2 gaps with no agents, got %d", len(gaps))
	}
	for _, g := range gaps {
		if g.Verdict != "uncovered" {
			t.Errorf("expected uncovered for %s with no agents, got %q", g.Domain, g.Verdict)
		}
	}
}

func TestFindGapsThresholdBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		score   float64
		verdict string
		isGap   bool
	}{
		{"score 0.0 → uncovered", 0.0, "uncovered", true},
		{"score 0.19 → uncovered", 0.19, "uncovered", true},
		{"score 0.2 → weakly_covered", 0.2, "weakly_covered", true},
		{"score 0.49 → weakly_covered", 0.49, "weakly_covered", true},
		{"score 0.5 → not a gap", 0.5, "", false},
		{"score 1.0 → not a gap", 1.0, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allDomains := map[string]bool{"testing": true}
			domainMap := map[string]map[string]float64{
				"agent": {"testing": tt.score},
			}

			gaps := FindGaps(allDomains, domainMap)

			if tt.isGap {
				if len(gaps) != 1 {
					t.Fatalf("expected 1 gap, got %d", len(gaps))
				}
				if gaps[0].Verdict != tt.verdict {
					t.Errorf("expected verdict %q, got %q", tt.verdict, gaps[0].Verdict)
				}
			} else {
				if len(gaps) != 0 {
					t.Errorf("expected no gap for score %.2f, got %+v", tt.score, gaps)
				}
			}
		})
	}
}

func TestFindGapsSortedOutput(t *testing.T) {
	allDomains := map[string]bool{
		"testing":  true,
		"backend":  true,
		"security": true,
	}
	domainMap := map[string]map[string]float64{
		"agent": {"testing": 0.0, "backend": 0.0, "security": 0.0},
	}

	gaps := FindGaps(allDomains, domainMap)

	if len(gaps) < 2 {
		t.Fatal("expected multiple gaps")
	}

	// Results should be sorted by domain name
	for i := 1; i < len(gaps); i++ {
		if gaps[i].Domain < gaps[i-1].Domain {
			t.Errorf("gaps not sorted: %q comes after %q", gaps[i].Domain, gaps[i-1].Domain)
		}
	}
}
