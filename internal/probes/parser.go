package probes

import (
	"regexp"
	"strconv"
	"strings"
)

// ParsedResponse holds parsed signals from a probe response.
type ParsedResponse struct {
	Confidence  *float64 // nil if not found
	HedgingScore float64
	IsRefusal    bool
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

	// Hedging
	textLower := strings.ToLower(raw)
	var maxHedging float64
	for _, hp := range hedgingPatterns {
		if hp.pattern.MatchString(textLower) && hp.weight > maxHedging {
			maxHedging = hp.weight
		}
	}
	result.HedgingScore = maxHedging

	// Refusal
	for _, rp := range refusalPatterns {
		if rp.MatchString(textLower) {
			result.IsRefusal = true
			break
		}
	}

	return result
}
