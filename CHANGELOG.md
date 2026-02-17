# Changelog

All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-02-16

### Added

- Recursive directory scanning (`-r, --recursive`) for nested agent repositories. Walks the entire directory tree and loads agents from all supported file types.
- Content-hash deduplication: identical agents (by SHA-256 of system prompt) are collapsed into a single representative with `also_found_in` metadata. Enabled by default with `--recursive`; disable with `--no-dedup`.
- Qualified IDs for name collisions: when different agents share a filename, IDs are prefixed with the relative directory path.
- JSON output enrichment: `content_hash`, `also_found_in`, `instance_count` per agent; `scan_metadata` block with total/unique/collapsed counts.
- Terminal report dedup summary line when duplicates are collapsed.

## [0.2.0] - 2026-02-15

### Added

- Goroutine panic recovery in probe runner — malformed LLM responses no longer crash the run.
- 429 rate limit retries with exponential backoff for Anthropic and OpenAI providers.
- Stderr warnings when agent files are skipped due to errors or missing system prompts.
- Pluggable domain definitions via `agent-evals.yaml` — select built-in domains, extend them with extra keywords, or define fully custom domains.

## [0.1.0] - 2026-02-14

### Added

- Static analysis engine: domain extraction across 18 recognized categories, pairwise overlap via Jaccard similarity and LCS-based prompt comparison, conflict detection, coverage gap identification, boundary and uncertainty language scoring.
- Live probe engine: generates boundary questions per agent, sends through LLM provider, scores refusal health, calibration, and stochastic consistency.
- Provider support for Anthropic, OpenAI, and OpenAI-compatible endpoints (Ollama, vLLM, etc.).
- Agent definition loader supporting YAML, JSON, Markdown with frontmatter, plain text, and directory-based agents.
- Configuration file auto-discovery (`agent-evals.yaml`) with domain filtering, threshold configuration, and provider defaults.
- Output formats: terminal with ANSI colors and pager, JSON for CI pipelines, Markdown for PR comments.
- CI mode (`--ci`) with JSON output, no pager, and configurable exit-code thresholds.
- Transcript output (`--transcript`) for full probe Q&A in Markdown format.
- Release infrastructure: GoReleaser cross-compilation, GitHub Actions CI/CD, Homebrew tap, curl install script.
