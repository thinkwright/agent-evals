# agent-evals — Feature Ideas & Research Directions

## Feature Development

### 1. Behavioral Trace Scoring

**Status**: Design doc complete ([design-behavioral-trace-scoring.md](design-behavioral-trace-scoring.md))

Adds three behavioral metrics computed from existing response text with zero additional API calls:

- **Confidence-Hedging Coherence** — Does the agent's language match its self-reported confidence number? Catches agents that write "I'm not sure" then report CONFIDENCE: 90.
- **Verbosity** — Word count per response. High verbosity on boundary probes (where the ideal answer is a concise hedge) is a signal of wasted compute.
- **Decisiveness** — How early in the response does the agent reach its signal (hedge, refuse, or answer)? Early signal = recognized the boundary quickly. Late signal = rambled first, hedged as afterthought.

Code changes are scoped to parser.go, scoring.go, runner.go, and the report formatters. No new CLI flags needed — behavioral scores appear automatically in `test` output.

### 2. LLM-Generated Probes for Custom Domains

Custom domains defined in config (e.g. `payments` with keywords `[stripe, plaid, ach]`) currently have no probe questions — `questions.go` just does `continue` for unknown domain keys. The tool could use the same LLM provider to generate boundary questions on the fly using the agent's system prompt and domain keywords as context. One meta-prompt, a handful of questions out, then run them like any other probe.

### 3. `diff` Command — Regression Detection Between Runs

No way to compare two runs today. A `diff` subcommand that takes two JSON report files and outputs what changed — score deltas, new overlaps, resolved conflicts, boundary regressions. Something like:

```
agent-evals diff baseline.json current.json
```

Exit 1 if any score regressed beyond a threshold. Makes the tool far more useful in CI pipelines.

### 4. Multi-Turn Boundary Probes

All probes are currently single-turn. Real boundary failures happen when an agent gets progressively led out of scope:

1. "Can you help with X?" (in-scope)
2. "Great, now what about the legal implications of X?" (drift)
3. "Draft me a legal disclaimer for X." (out-of-scope)

A small multi-turn probe type (2-3 turns, starting in-scope and drifting out) would catch agents that fail to maintain boundaries under conversational pressure.

---

## Research Directions

### The Say-Do Gap (Existing Finding)

The Agent Census study was run with this codebase against the wshobson agent collection — 496 files scanned, 428 unique agents after SHA-256 content-hash deduplication, 10,000 API calls across 2,500 probes.

**Data files**:

- `~/Dev/Metacognition/wshobson-analysis.json` (35MB) — static-only run (2026-02-16)
- `~/Dev/Metacognition/agent-census/census-analysis.json` (35MB) — full run with live probes (2026-02-17)
- `~/Dev/Metacognition/agent-census/census-transcript.md` (17MB) — raw LLM responses

Same 428 agents in both files; the census file adds `live_scores` and `live_summary` on top of the static analysis.

**Static analysis findings** (428 agents):

| Metric | Mean | Median |
|---|---|---|
| Boundary definition score | 0.59 | 0.70 |
| Scope clarity score | 0.84 | 1.00 |
| Uncertainty guidance score | 0.35 | 0.30 |

- 72% have boundary language, but only 9% have uncertainty guidance
- Agents are clear about what they *do*, but almost never say what they *don't* do
- 74,714 pairwise overlaps: 66,406 clean, 8,037 warnings, 271 conflicts
- 37,571 issues: 280 errors, 36,785 warnings, 506 info
- Top strong domains: databases (69%), testing (50%), backend (49%)

**Live probe findings** (420 agents probed, 2,500 probes, 10,000 API calls):

| Metric | Mean | Median | Stdev |
|---|---|---|---|
| Boundary score | 0.293 | 0.333 | 0.121 |
| Calibration score | 0.960 | 1.000 | 0.145 |
| Consistency score | 0.829 | 0.929 | 0.292 |
| Refusal health | 0.042 | 0.000 | 0.091 |

The core finding: agents score high on calibration (they answer in-scope questions well) and consistency (they're reliably wrong, not randomly wrong), but boundary scores are low and refusal health is near zero. They confidently answer out-of-scope questions despite boundary instructions.

### Research Angle 1: Coherence as a Reliability Signal (Strongest)

Nobody is systematically measuring whether agents' self-reported confidence matches their linguistic behavior. The research question:

> Do LLM agents exhibit confidence-hedging incoherence, and does incoherence predict incorrect answers?

Validate by running calibration probes where ground truth is known, then correlating coherence score with factual accuracy. If low coherence predicts errors, that's a cheap proxy signal for reliability requiring no ground-truth labels at inference time.

### Research Angle 2: Empirical Study — Boundary Awareness in the Wild

Collect real agent configurations from open-source projects, run agent-evals with behavioral scoring across multiple models, and report findings:

- Do agents with explicit boundary language actually refuse more appropriately?
- Does boundary awareness degrade at domain boundaries (the overlap zones)?
- How do scores differ across model families/sizes?

This is a measurement paper — the tool is the methodology section. The kind that EMNLP, ACL workshops, and NeurIPS agent workshops accept.

### Research Angle 3: Prompt Engineering for Metacognitive Calibration

Use the behavioral metrics as an optimization target: which prompt patterns produce the most coherent, decisive, well-calibrated agents? Test interventions like explicit confidence instructions, role framing, boundary enumeration, and measure their effect on coherence and decisiveness scores.

### Retroactive Analysis Opportunity

The raw data from the Agent Census (17MB transcript in `census-transcript.md`, 35MB structured output in `census-analysis.json`) already contains every raw response needed to compute behavioral trace scores retroactively — no need to re-run the probes. This means behavioral scoring can be validated against the existing corpus immediately.

**Title direction**: *"Confidence Without Calibration: How LLM Agents Fail at Scope Boundaries"*

**Core claims the data could support**:

1. The coherence gap is systematic across the agent ecosystem
2. Failure modes are distinguishable (confident-and-wrong vs. verbose deflection vs. late hedge)
3. Behavioral signals predict boundary failure better than content-level signals alone
