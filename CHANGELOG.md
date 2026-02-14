# Changelog

All notable changes to this project will be documented in this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-02-14

### Added

- Static analysis engine: domain extraction across 19 recognized categories, pairwise overlap via Jaccard similarity and LCS-based prompt comparison, conflict detection, coverage gap identification, boundary and uncertainty language scoring.
- Live probe engine: generates boundary questions per agent, sends through LLM provider, scores refusal health, calibration, and stochastic consistency.
- Provider support for Anthropic, OpenAI, and OpenAI-compatible endpoints (Ollama, vLLM, etc.).
- Agent definition loader supporting YAML, JSON, Markdown with frontmatter, plain text, and directory-based agents.
- Configuration file auto-discovery (`agent-evals.yaml`) with domain filtering, threshold configuration, and provider defaults.
- Output formats: terminal with ANSI colors and pager, JSON for CI pipelines, Markdown for PR comments.
- CI mode (`--ci`) with JSON output, no pager, and configurable exit-code thresholds.
- Transcript output (`--transcript`) for full probe Q&A in Markdown format.
- Release infrastructure: GoReleaser cross-compilation, GitHub Actions CI/CD, Homebrew tap, curl install script.
