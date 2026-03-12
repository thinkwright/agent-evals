package probes

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// RescoreReport reads an existing JSON report and optionally a transcript
// file, re-parses raw LLM responses to compute behavioral trace metrics
// (coherence, decisiveness, word count), and returns the enriched JSON.
// No API calls are made.
func RescoreReport(reportPath, transcriptPath string) ([]byte, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}

	var report map[string]any
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse report: %w", err)
	}

	agents, ok := report["agents"].([]any)
	if !ok {
		return nil, fmt.Errorf("report has no 'agents' array")
	}

	// Try inline live_details first
	rescored := 0
	for _, raw := range agents {
		agent, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		liveScores, ok := agent["live_scores"].(map[string]any)
		if !ok {
			continue
		}

		details, ok := agent["live_details"].([]any)
		if !ok {
			continue
		}

		results := rescoreFromDetails(details)
		liveScores["coherence_score"] = results.CoherenceScore
		liveScores["decisiveness_score"] = results.DecisivenessScore
		liveScores["mean_word_count"] = round3f(results.MeanWordCount)
		rescored++
	}

	if rescored > 0 {
		return marshalReport(report)
	}

	// Try transcript file
	if transcriptPath != "" {
		return rescoreFromTranscript(report, agents, transcriptPath)
	}

	fmt.Fprintf(os.Stderr, "Warning: report has no live_details and no transcript provided; returning original data\n")
	return data, nil
}

// rescoreFromDetails re-parses raw responses within a single agent's live_details.
func rescoreFromDetails(details []any) *AgentProbeResults {
	results := &AgentProbeResults{}

	for _, rawDetail := range details {
		detail, ok := rawDetail.(map[string]any)
		if !ok {
			continue
		}

		probeType, _ := detail["probe_type"].(string)
		responses, ok := detail["responses"].([]any)
		if !ok {
			continue
		}

		pd := ProbeDetail{ProbeType: probeType}
		for _, rawResp := range responses {
			resp, ok := rawResp.(map[string]any)
			if !ok {
				continue
			}

			rawText, _ := resp["raw"].(string)
			if rawText == "" {
				continue
			}

			temp, _ := resp["temperature"].(float64)
			parsed := ParseProbeResponse(rawText)
			pd.Responses = append(pd.Responses, ResponseRecord{
				Run:             intFromAny(resp["run"]),
				Temperature:     temp,
				Confidence:      parsed.Confidence,
				HedgingScore:    parsed.HedgingScore,
				IsRefusal:       parsed.IsRefusal,
				Raw:             rawText,
				CoherenceScore:  parsed.CoherenceScore,
				WordCount:       parsed.WordCount,
				DecisivenessPos: parsed.DecisivenessPos,
			})
		}

		results.Details = append(results.Details, pd)
	}

	ScoreAgentProbes(results)
	return results
}

func marshalReport(report map[string]any) ([]byte, error) {
	enriched, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal enriched report: %w", err)
	}
	return enriched, nil
}

// rescoreFromTranscript parses a FormatTranscript-style markdown file,
// re-scores each agent's responses, and merges behavioral metrics into
// the JSON report.
func rescoreFromTranscript(report map[string]any, agents []any, transcriptPath string) ([]byte, error) {
	agentResults, err := parseTranscript(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("parse transcript: %w", err)
	}

	// Build agent ID → live_scores lookup
	agentMap := make(map[string]map[string]any)
	for _, raw := range agents {
		agent, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := agent["id"].(string)
		ls, _ := agent["live_scores"].(map[string]any)
		if id != "" && ls != nil {
			agentMap[id] = ls
		}
	}

	rescored := 0
	for agentID, results := range agentResults {
		ScoreAgentProbes(results)
		ls, ok := agentMap[agentID]
		if !ok {
			continue
		}
		ls["coherence_score"] = round3f(results.CoherenceScore)
		ls["decisiveness_score"] = round3f(results.DecisivenessScore)
		ls["mean_word_count"] = round3f(results.MeanWordCount)
		rescored++
	}

	fmt.Fprintf(os.Stderr, "Rescored %d agents from transcript\n", rescored)
	return marshalReport(report)
}

var (
	reAgent    = regexp.MustCompile(`^## (.+)$`)
	reProbe    = regexp.MustCompile(`^### Probe \d+: \S+ \((\w+)\)$`)
	reResponse = regexp.MustCompile(`^#### Response \((.+)\)`)
	reTempRun  = regexp.MustCompile(`T=([\d.]+), run (\d+)`)
)

// parseTranscript reads a FormatTranscript markdown file and returns
// per-agent probe results with raw response text.
//
// LLM responses may contain bare ``` fences, so we can't rely on code
// fence toggling. Instead we collect raw text between the first ```
// after a #### Response header and the next structural marker (##, ###,
// ####, ---, or end of file), stripping the outer fences.
func parseTranscript(path string) (map[string]*AgentProbeResults, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	results := make(map[string]*AgentProbeResults)
	var (
		agentID       string
		currentDetail *ProbeDetail
		temp          float64
		run           int
		collecting    bool   // inside a response block (after #### Response)
		inCode        bool   // saw the opening ```
		codeLines     []string
	)

	flushResponse := func() {
		if !inCode || currentDetail == nil || len(codeLines) == 0 {
			codeLines = nil
			inCode = false
			collecting = false
			return
		}
		rawText := strings.Join(codeLines, "\n")
		parsed := ParseProbeResponse(rawText)
		currentDetail.Responses = append(currentDetail.Responses, ResponseRecord{
			Run:             run,
			Temperature:     temp,
			Confidence:      parsed.Confidence,
			HedgingScore:    parsed.HedgingScore,
			IsRefusal:       parsed.IsRefusal,
			Raw:             rawText,
			CoherenceScore:  parsed.CoherenceScore,
			WordCount:       parsed.WordCount,
			DecisivenessPos: parsed.DecisivenessPos,
		})
		codeLines = nil
		inCode = false
		collecting = false
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Structural markers end any in-progress response collection
		isStructural := strings.HasPrefix(line, "## ") ||
			strings.HasPrefix(line, "### ") ||
			strings.HasPrefix(line, "#### ") ||
			line == "---"

		if collecting && isStructural {
			flushResponse()
			// fall through to handle the structural line
		}

		// Agent header: ## agent-id
		if m := reAgent.FindStringSubmatch(line); m != nil {
			agentID = m[1]
			if _, ok := results[agentID]; !ok {
				results[agentID] = &AgentProbeResults{AgentID: agentID}
			}
			currentDetail = nil
			continue
		}

		// Probe header: ### Probe N: probe_id (type)
		if m := reProbe.FindStringSubmatch(line); m != nil && agentID != "" {
			detail := ProbeDetail{ProbeType: m[1]}
			results[agentID].Details = append(results[agentID].Details, detail)
			currentDetail = &results[agentID].Details[len(results[agentID].Details)-1]
			continue
		}

		// Response header: #### Response (deterministic) or #### Response (T=0.7, run 1)
		if m := reResponse.FindStringSubmatch(line); m != nil {
			label := m[1]
			collecting = true
			inCode = false
			codeLines = nil
			if strings.HasPrefix(label, "deterministic") {
				temp = 0
				run = 0
			} else if tm := reTempRun.FindStringSubmatch(label); tm != nil {
				temp, _ = strconv.ParseFloat(tm[1], 64)
				run, _ = strconv.Atoi(tm[2])
			}
			continue
		}

		// Inside response collection
		if collecting {
			if !inCode {
				// Wait for opening fence
				if strings.HasPrefix(line, "```") {
					inCode = true
				}
				continue
			}
			// Accumulate code lines
			codeLines = append(codeLines, line)
		}
	}
	// Flush any trailing response
	flushResponse()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan transcript: %w", err)
	}

	return results, nil
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func floatFromAny(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func round3f(f float64) float64 {
	return float64(int(f*1000+0.5)) / 1000
}
