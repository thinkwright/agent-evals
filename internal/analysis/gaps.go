package analysis

import "sort"

// GapResult represents a domain with insufficient agent coverage.
type GapResult struct {
	Domain       string
	ClosestAgent string
	ClosestScore float64
	Verdict      string // "uncovered" | "weakly_covered"
}

// FindGaps finds domains with no strong agent coverage.
func FindGaps(allDomains map[string]bool, domainMap map[string]map[string]float64) []GapResult {
	sorted := make([]string, 0, len(allDomains))
	for d := range allDomains {
		sorted = append(sorted, d)
	}
	sort.Strings(sorted)

	var gaps []GapResult
	for _, domain := range sorted {
		var bestAgent string
		var bestScore float64

		for agentID, scores := range domainMap {
			score := scores[domain]
			if score > bestScore {
				bestScore = score
				bestAgent = agentID
			}
		}

		if bestScore < 0.2 {
			gaps = append(gaps, GapResult{
				Domain:       domain,
				ClosestAgent: bestAgent,
				ClosestScore: bestScore,
				Verdict:      "uncovered",
			})
		} else if bestScore < 0.5 {
			gaps = append(gaps, GapResult{
				Domain:       domain,
				ClosestAgent: bestAgent,
				ClosestScore: bestScore,
				Verdict:      "weakly_covered",
			})
		}
	}

	return gaps
}
