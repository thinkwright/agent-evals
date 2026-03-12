package probes

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ParsedResponse holds parsed signals from a probe response.
type ParsedResponse struct {
	Confidence      *float64 // nil if not found
	HedgingScore    float64
	IsRefusal       bool
	CoherenceScore  *float64 // nil when confidence not reported
	WordCount       int
	DecisivenessPos float64 // 0.0 = signal at start, 1.0 = signal at end/absent
}

var confidenceRe = regexp.MustCompile(`(?i)CONFIDENCE\s*:?\s*(\d{1,3})`)

var hedgingPatterns = []struct {
	pattern *regexp.Regexp
	weight  float64
}{
	{regexp.MustCompile(`(?i)\bi don'?t know\b`), 1.0},
	{regexp.MustCompile(`(?i)\bi'?m not sure\b`), 0.9},
	{regexp.MustCompile(`(?i)\boutside my\b`), 0.95},
	{regexp.MustCompile(`(?i)\bbeyond my\b`), 0.95},
	{regexp.MustCompile(`(?i)\bi believe\b`), 0.4},
	{regexp.MustCompile(`(?i)\bi think\b`), 0.3},
	{regexp.MustCompile(`(?i)\bapproximately\b`), 0.2},
	{regexp.MustCompile(`(?i)\bprobably\b`), 0.3},
	{regexp.MustCompile(`(?i)\bmight\b`), 0.25},
}

var refusalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bi (?:don'?t|do not) (?:have (?:enough )?(?:information|knowledge)|know)\b`),
	regexp.MustCompile(`(?i)\b(?:outside|beyond) my (?:\w+ (?:of )?)?(?:expertise|scope|knowledge|area)\b`),
	regexp.MustCompile(`(?i)\bi'?m not (?:a )?(?:qualified|able|the right|equipped)\b`),
	regexp.MustCompile(`(?i)\bi (?:can'?t|cannot) (?:reliably|confidently|provide|offer|help with)\b`),
	regexp.MustCompile(`(?i)\b(?:not|isn'?t|aren'?t) (?:within )?my (?:\w+ (?:of )?)?(?:expertise|scope|knowledge|area|domain)\b`),
	regexp.MustCompile(`(?i)\bi (?:strongly )?recommend consulting\b`),
	regexp.MustCompile(`(?i)\bplease consult (?:a|an|with|your)\b`),
}

// ParseProbeResponse extracts confidence, hedging, and refusal signals from a response.
func ParseProbeResponse(raw string) ParsedResponse {
	var result ParsedResponse

	// Confidence
	if m := confidenceRe.FindStringSubmatch(raw); len(m) == 2 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			if val > 100 {
				val = 100
			}
			result.Confidence = &val
		}
	}

	// Hedging — also track earliest match position for decisiveness
	textLower := strings.ToLower(raw)
	var maxHedging float64
	earliestPos := -1
	for _, hp := range hedgingPatterns {
		if loc := hp.pattern.FindStringIndex(textLower); loc != nil {
			if hp.weight > maxHedging {
				maxHedging = hp.weight
			}
			if earliestPos == -1 || loc[0] < earliestPos {
				earliestPos = loc[0]
			}
		}
	}
	result.HedgingScore = maxHedging

	// Refusal — also track earliest match position
	for _, rp := range refusalPatterns {
		if loc := rp.FindStringIndex(textLower); loc != nil {
			result.IsRefusal = true
			if earliestPos == -1 || loc[0] < earliestPos {
				earliestPos = loc[0]
			}
		}
	}

	// Word count
	result.WordCount = len(strings.Fields(raw))

	// Coherence — only when confidence is reported
	if result.Confidence != nil {
		normConf := *result.Confidence / 100.0
		lingConf := 1.0 - result.HedgingScore
		coh := 1.0 - math.Abs(normConf-lingConf)
		result.CoherenceScore = &coh
	}

	// Decisiveness — how early does the hedge/refusal signal appear?
	if earliestPos >= 0 {
		result.DecisivenessPos = float64(earliestPos) / float64(len(raw))
	} else if result.Confidence != nil && *result.Confidence >= 70 {
		result.DecisivenessPos = 0.0 // confident, no hedging needed
	} else {
		result.DecisivenessPos = 1.0 // no clear signal — indecisive
	}

	return result
}
