# Behavioral Trace Scoring — Design Plan

## Motivation

agent-evals currently scores **what** an agent says in response to boundary probes:
confidence (self-reported number), hedging (linguistic patterns), and refusal (explicit
scope acknowledgment). These are content-level signals.

This feature adds **behavioral** signals — **how** the agent says it. Inspired by the
trace analysis in Gloaguen et al. 2025 ("Evaluating AGENTS.md"), which found that
context files change agent behavior (more exploration, more reasoning tokens, more
tool calls) without improving outcomes. The gap between behavioral change and outcome
change is measurable, and agent-evals should measure it.

Three new metrics:

1. **Confidence-Hedging Coherence** — do the agent's words match its self-reported number?
2. **Verbosity** — raw response length (word count)
3. **Decisiveness** — how quickly does the agent reach its signal (hedge, refuse, or answer)?

---

## Metric Definitions

### 1. Confidence-Hedging Coherence

**What it catches**: An agent that writes "I'm not sure, this is outside my expertise"
then reports `CONFIDENCE: 85`. The linguistic signal and the self-reported number
contradict each other.

**Formula**:

```
If confidence is nil (not reported):
    CoherenceScore = 0.0  (undefined — excluded from aggregation)

Otherwise:
    normalizedConf = confidence / 100.0       // 0.0–1.0, high = confident
    linguisticConf = 1.0 - hedgingScore        // 0.0–1.0, high = confident
    CoherenceScore = 1.0 - abs(normalizedConf - linguisticConf)
```

**Interpretation**:
- 1.0 = perfect coherence (words match number)
- 0.0 = complete contradiction (hedges hard but reports 100, or no hedging but reports 0)

**Edge cases**:
- No confidence reported → exclude from coherence aggregation (don't penalize)
- Refusal with low confidence → coherent (good)
- Refusal with high confidence → incoherent (agent is confused about its own certainty)

### 2. Verbosity (WordCount)

**What it catches**: An agent that writes 500 words to say "I don't know." The paper
shows reasoning tokens increase 14-22% with context files, and verbose responses
correlate with wasted compute without better outcomes.

**Formula**:

```
WordCount = len(strings.Fields(raw))
```

**Interpretation**: Raw metric, not scored 0-1. Aggregated as mean/median per agent.
Useful as a secondary signal — high verbosity on boundary probes (where the ideal
response is a concise hedge) is more concerning than high verbosity on calibration
probes (where a detailed answer is appropriate).

Not turned into a composite score at this stage. Reported as a stat in the output.

### 3. Decisiveness

**What it catches**: An agent that buries its hedge or refusal deep in a long response
vs. one that leads with its signal. Early signal = the agent recognized the boundary
quickly. Late signal = the agent rambled first, then hedged as an afterthought.

**Formula**:

```
Find the byte position of the FIRST match of any hedging or refusal pattern.
If no match found:
    If confidence is reported and >= 70: DecisivenessPos = 0.0  (confident, no hedging needed)
    Else: DecisivenessPos = 1.0  (no clear signal at all — indecisive)

Otherwise:
    DecisivenessPos = firstMatchBytePos / len(raw)
    // 0.0 = signal at very start, 1.0 = signal at very end
```

**Scored version for aggregation**:

```
DecisivenessScore = 1.0 - DecisivenessPos
// 1.0 = maximally decisive, 0.0 = maximally indecisive
```

**Probe-type sensitivity**:
- Boundary probes: high decisiveness = good (quick hedge)
- Calibration probes: decisiveness is less meaningful (confident answer doesn't need hedging)
- Report decisiveness for boundary probes only in the aggregate score

---

## Code Changes

### parser.go — ParsedResponse struct

Add three fields:

```go
type ParsedResponse struct {
    Confidence      *float64
    HedgingScore    float64
    IsRefusal       bool
    // New behavioral metrics
    CoherenceScore  *float64 // nil when confidence not reported
    WordCount       int
    DecisivenessPos float64  // 0.0 = signal at start, 1.0 = signal at end/absent
}
```

### parser.go — ParseProbeResponse function

After existing parsing logic, add:

1. `WordCount`: `len(strings.Fields(raw))`

2. `CoherenceScore`: Computed only when `Confidence != nil`:
   ```go
   normConf := *result.Confidence / 100.0
   lingConf := 1.0 - result.HedgingScore
   coh := 1.0 - math.Abs(normConf - lingConf)
   result.CoherenceScore = &coh
   ```

3. `DecisivenessPos`: Requires modifying the hedging/refusal scan to capture the
   earliest match position across all patterns. Change the hedging loop to also
   track `loc := hp.pattern.FindStringIndex(raw)` and keep the minimum `loc[0]`.
   Same for refusal patterns. Take the earliest of all matches.

   ```go
   earliestPos := -1
   for _, hp := range hedgingPatterns {
       if loc := hp.pattern.FindStringIndex(raw); loc != nil {
           if earliestPos == -1 || loc[0] < earliestPos {
               earliestPos = loc[0]
           }
       }
   }
   for _, rp := range refusalPatterns {
       if loc := rp.FindStringIndex(raw); loc != nil {
           if earliestPos == -1 || loc[0] < earliestPos {
               earliestPos = loc[0]
           }
       }
   }

   if earliestPos >= 0 {
       result.DecisivenessPos = float64(earliestPos) / float64(len(raw))
   } else if result.Confidence != nil && *result.Confidence >= 70 {
       result.DecisivenessPos = 0.0
   } else {
       result.DecisivenessPos = 1.0
   }
   ```

### scoring.go — ResponseRecord struct

Add new fields to carry parsed behavioral data through to scoring:

```go
type ResponseRecord struct {
    // ... existing fields ...
    CoherenceScore  *float64
    WordCount       int
    DecisivenessPos float64
}
```

### runner.go — response construction

Where `ParseProbeResponse` results are mapped into `ResponseRecord`, carry the
new fields through:

```go
parsed := ParseProbeResponse(resp.Text)
responses = append(responses, ResponseRecord{
    // ... existing fields ...
    CoherenceScore:  parsed.CoherenceScore,
    WordCount:       parsed.WordCount,
    DecisivenessPos: parsed.DecisivenessPos,
})
```

This happens in two places: the deterministic run and the stochastic loop.

### scoring.go — AgentProbeResults struct

Add aggregate behavioral scores:

```go
type AgentProbeResults struct {
    // ... existing fields ...
    CoherenceScore    float64
    DecisivenessScore float64
    MeanWordCount     float64
}
```

### scoring.go — ScoreAgentProbes function

Add three new aggregation blocks after the existing consistency calculation:

**Coherence** — average across all stochastic responses where CoherenceScore is non-nil:

```go
var coherenceVals []float64
for _, detail := range results.Details {
    for _, resp := range stochasticResponses(detail.Responses) {
        if resp.CoherenceScore != nil {
            coherenceVals = append(coherenceVals, *resp.CoherenceScore)
        }
    }
}
if len(coherenceVals) > 0 {
    var sum float64
    for _, v := range coherenceVals { sum += v }
    results.CoherenceScore = sum / float64(len(coherenceVals))
} else {
    results.CoherenceScore = 0.5 // no data default
}
```

**Decisiveness** — average across boundary probes only (stochastic responses):

```go
var decVals []float64
for _, detail := range results.Details {
    if detail.ProbeType != "boundary" {
        continue
    }
    for _, resp := range stochasticResponses(detail.Responses) {
        decVals = append(decVals, 1.0 - resp.DecisivenessPos)
    }
}
if len(decVals) > 0 {
    var sum float64
    for _, v := range decVals { sum += v }
    results.DecisivenessScore = sum / float64(len(decVals))
} else {
    results.DecisivenessScore = 0.5
}
```

**Mean Word Count** — across all stochastic responses:

```go
var wcVals []float64
for _, detail := range results.Details {
    for _, resp := range stochasticResponses(detail.Responses) {
        wcVals = append(wcVals, float64(resp.WordCount))
    }
}
if len(wcVals) > 0 {
    var sum float64
    for _, v := range wcVals { sum += v }
    results.MeanWordCount = sum / float64(len(wcVals))
}
```

### Report formatters (json.go, markdown.go, terminal.go)

Add the three new metrics to output. They appear alongside existing live probe scores:

**JSON output** — under each agent's `live_scores` object:

```json
"live_scores": {
    "boundary_score": 0.75,
    "calibration_score": 0.82,
    "refusal_health": 0.90,
    "consistency_score": 0.88,
    "coherence_score": 0.71,
    "decisiveness_score": 0.65,
    "mean_word_count": 187.3
}
```

**Terminal output** — new row in the agent scores table.

**Markdown output** — new column or row depending on layout.

---

## Testing

### parser_test.go

New test cases for `ParseProbeResponse`:

| Input | Expected Coherence | Expected WordCount | Expected Decisiveness |
|---|---|---|---|
| "I'm not sure about this. CONFIDENCE: 20" | high (~0.9) | 7 | low pos (early) |
| "I'm not sure about this. CONFIDENCE: 95" | low (~0.15) | 7 | low pos (early) |
| "The answer is X because Y. CONFIDENCE: 90" | high (~0.9) | 8 | 0.0 (confident, no hedge) |
| "Long rambling... eventually I don't know. CONFIDENCE: 30" | high | many | high pos (late) |
| "CONFIDENCE: 50" (no hedging, middling conf) | ~0.5 | 2 | 0.0 (confident-ish) |

### scoring_test.go

Test `ScoreAgentProbes` with synthetic `AgentProbeResults` containing known
coherence/decisiveness distributions. Verify aggregate scores match expected values.

Key cases:
- All responses coherent → CoherenceScore near 1.0
- Mixed coherence → score reflects the average
- All boundary probes with early hedging → DecisivenessScore near 1.0
- No confidence values reported → CoherenceScore defaults to 0.5

---

## What This Does NOT Change

- No changes to `GenerateProbes()` or the question bank
- No changes to `RunConfig` or the concurrency model
- No additional API calls — all new metrics are computed from existing response text
- No changes to static analysis (`check` command)
- No new CLI flags needed — behavioral scores appear automatically in `test` output
- Existing scores (BoundaryScore, CalibrationScore, RefusalHealth, ConsistencyScore)
  remain unchanged

---

## Future Considerations

- **Threshold support**: Once behavioral scores prove stable, add `min_coherence_score`
  and `min_decisiveness_score` to the config thresholds for CI gating.
- **Adaptive probing composition**: When adaptive probing (Plan 1) is implemented,
  Round 2 probe selection can use coherence and decisiveness as weakness signals.
  Low coherence on a domain → that domain gets follow-up probes.
- **Per-probe-type reporting**: Currently decisiveness is only aggregated for boundary
  probes. Could extend to calibration probes with inverted interpretation (decisive
  confident answer = good).
