package analysis

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/thinkwright/agent-evals/internal/loader"
)

// OverlapResult represents pairwise overlap between two agents.
type OverlapResult struct {
	AgentA                  string
	AgentB                  string
	SharedDomains           []string
	OverlapScore            float64 // 0-1 Jaccard similarity
	PromptSimilarity        float64 // 0-1 textual similarity
	ConflictingInstructions []string
	Verdict                 string // "clean" | "warning" | "conflict"
}

// ComputeOverlaps computes pairwise overlap between all agents.
func ComputeOverlaps(agents []loader.AgentDefinition, domainMap map[string]map[string]float64) []OverlapResult {
	var results []OverlapResult
	for i := 0; i < len(agents); i++ {
		for j := i + 1; j < len(agents); j++ {
			results = append(results, computeOverlap(&agents[i], &agents[j], domainMap))
		}
	}
	return results
}

func computeOverlap(a, b *loader.AgentDefinition, domainMap map[string]map[string]float64) OverlapResult {
	domainsA := strongDomains(domainMap[a.ID], 0.3)
	domainsB := strongDomains(domainMap[b.ID], 0.3)

	shared := intersection(domainsA, domainsB)
	all := union(domainsA, domainsB)

	var overlapScore float64
	if len(all) > 0 {
		overlapScore = float64(len(shared)) / float64(len(all))
	}

	promptSim := similarity(truncate(strings.ToLower(a.SystemPrompt), 2000),
		truncate(strings.ToLower(b.SystemPrompt), 2000))

	conflicts := detectConflicts(a, b)

	verdict := "clean"
	if len(conflicts) > 0 {
		verdict = "conflict"
	} else if overlapScore > 0.5 {
		verdict = "warning"
	}

	sortedShared := make([]string, 0, len(shared))
	for d := range shared {
		sortedShared = append(sortedShared, d)
	}
	sort.Strings(sortedShared)

	return OverlapResult{
		AgentA:                  a.ID,
		AgentB:                  b.ID,
		SharedDomains:           sortedShared,
		OverlapScore:            overlapScore,
		PromptSimilarity:        promptSim,
		ConflictingInstructions: conflicts,
		Verdict:                 verdict,
	}
}

// Opposition pairs for conflict detection.
var oppositionPairs = []struct {
	positive string
	negative string
}{
	{`always use (\w+)`, `(?:never|avoid|don't) use %s`},
	{`prefer (\w+)`, `(?:avoid|don't use|never) %s`},
	{`must (?:always )?(\w+)`, `(?:must not|should not|never) %s`},
	{`use (\w+) for`, `(?:don't|never|avoid) (?:using )?%s for`},
}

func detectConflicts(a, b *loader.AgentDefinition) []string {
	textA := strings.ToLower(a.FullContext())
	textB := strings.ToLower(b.FullContext())

	seen := make(map[string]bool)
	var conflicts []string

	check := func(srcID, dstID, srcText, dstText string) {
		for _, pair := range oppositionPairs {
			re := regexp.MustCompile(pair.positive)
			matches := re.FindAllStringSubmatch(srcText, -1)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				captured := m[1]
				negPattern := fmt.Sprintf(pair.negative, regexp.QuoteMeta(captured))
				negRe, err := regexp.Compile(negPattern)
				if err != nil {
					continue
				}
				if negRe.MatchString(dstText) {
					msg := fmt.Sprintf("'%s' says use '%s' but '%s' says avoid it", srcID, captured, dstID)
					if !seen[msg] {
						seen[msg] = true
						conflicts = append(conflicts, msg)
					}
				}
			}
		}
	}

	check(a.ID, b.ID, textA, textB)
	check(b.ID, a.ID, textB, textA)

	return conflicts
}

// helpers

func strongDomains(scores map[string]float64, threshold float64) map[string]bool {
	result := make(map[string]bool)
	for d, s := range scores {
		if s > threshold {
			result[d] = true
		}
	}
	return result
}

func intersection(a, b map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for k := range a {
		if b[k] {
			result[k] = true
		}
	}
	return result
}

func union(a, b map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for k := range a {
		result[k] = true
	}
	for k := range b {
		result[k] = true
	}
	return result
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// similarity computes a simple character-level similarity ratio between two strings.
// This is a basic implementation similar to Python's SequenceMatcher.ratio().
func similarity(a, b string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	// LCS-based similarity
	m := len(a)
	n := len(b)

	// Use two rows for space efficiency
	prev := make([]int, n+1)
	curr := make([]int, n+1)

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] > curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
		for k := range curr {
			curr[k] = 0
		}
	}

	lcs := prev[n]
	return 2.0 * float64(lcs) / float64(m+n)
}
