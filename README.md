# agent-evals

[![CI](https://github.com/thinkwright/agent-evals/actions/workflows/ci.yaml/badge.svg)](https://github.com/thinkwright/agent-evals/actions/workflows/ci.yaml)
[![Release](https://img.shields.io/github/v/release/thinkwright/agent-evals?color=blue)](https://github.com/thinkwright/agent-evals/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/thinkwright/agent-evals.svg)](https://pkg.go.dev/github.com/thinkwright/agent-evals)
[![Downloads](https://img.shields.io/github/downloads/thinkwright/agent-evals/total?color=orange)](https://github.com/thinkwright/agent-evals/releases)
[![Homebrew](https://img.shields.io/badge/homebrew-thinkwright%2Ftap-blueviolet)](https://github.com/thinkwright/homebrew-tap)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Static analysis and live boundary testing for LLM agent configurations. Detects scope overlap between agents, scores boundary awareness and calibration, identifies coverage gaps, and verifies that agents actually refuse out-of-scope questions at inference time.

### Works With

Claude Code · Cline · Cursor · Augment · Windsurf · Copilot · Aider · Custom YAML / JSON / Markdown agents

agent-evals reads system prompts and agent definitions from any coding agent that uses text-based configuration. If your agent has a system prompt, agent-evals can analyze it.

For full documentation, visit [thinkwright.ai/agent-evals](https://thinkwright.ai/agent-evals). For a live probe example with annotated results, see [thinkwright.ai/agent-evals/example](https://thinkwright.ai/agent-evals/example).

## Install

```sh
# Go
go install github.com/thinkwright/agent-evals/cmd/agent-evals@latest

# Homebrew
brew install thinkwright/tap/agent-evals

# curl
curl -sSL https://thinkwright.ai/install | sh
```

## Quick Start

agent-evals reads agent definitions from a directory and runs analysis against them. At minimum, each agent needs an identifier and a system prompt.

```sh
# Static analysis only (no API calls, no credentials)
agent-evals check ./agents/

# Static analysis + live boundary probes
agent-evals test ./agents/ --provider anthropic
```

The `check` command extracts domains from each agent's system prompt, computes pairwise overlap using Jaccard similarity and LCS-based prompt comparison, flags conflicts between overlapping agents, identifies coverage gaps across 19 recognized domain categories, and scores boundary awareness. It requires no API keys or network access.

The `test` command runs everything in `check`, then generates boundary questions tailored to each agent and sends them through your LLM provider. It measures whether agents hedge on out-of-scope questions, whether their self-reported confidence tracks actual capability, and whether responses stay consistent across repeated stochastic runs.

## Agent Definitions

agent-evals supports several formats for defining agents. Place agent files in a directory and point the tool at it.

```yaml
# agents/backend_api.yaml
id: backend_api
name: Backend API Engineer
system_prompt: |
  You are a senior backend API engineer specializing in RESTful services,
  PostgreSQL optimization, and microservices architecture with Go and Java.
  If a question falls outside backend development, say so clearly and
  suggest the user consult a relevant specialist.
```

Supported formats include YAML, JSON, Markdown with frontmatter, plain text files, and directory-based agents where `AGENT.md`, `RULES.md`, and `SKILLS.md` files are combined into a single definition. The loader accepts fields named `system_prompt`, `prompt`, `system`, `instructions`, or `content` for the agent's prompt text.

## Configuration

Place an `agent-evals.yaml` file alongside your agent definitions, or pass `--config` to specify a path. Configuration is optional; defaults work for most cases.

```yaml
# agent-evals.yaml
domains:
  - backend
  - frontend
  - databases
  - security
  # Extend a built-in domain with extra keywords
  - name: backend
    extends: builtin
    keywords: [axum, actix-web, tokio]
  # Add a fully custom domain
  - name: payments
    keywords: [payment gateway, stripe, plaid, ach transfer]

thresholds:
  min_overall_score: 0.7
  min_boundary_score: 0.5

probes:
  provider: anthropic
  model: claude-sonnet-4-5-20250514
  api_key_env: ANTHROPIC_API_KEY
```

The `domains` field configures which domains to analyze. Entries can be strings (built-in references), maps that extend a built-in with extra keywords (`extends: builtin`), or fully custom domains with their own keyword lists. Omit `domains` to use all 18 built-in domains. See [DOMAINS.md](DOMAINS.md) for the full list and customization details. The `thresholds` section controls CI exit codes when using `--ci`. The `probes` section provides defaults for provider, model, and API key configuration, which can be overridden by CLI flags.

## Providers

Live probes support three provider configurations.

```sh
# Anthropic (default)
export ANTHROPIC_API_KEY=sk-ant-...
agent-evals test ./agents/ --provider anthropic --model claude-sonnet-4-5-20250514

# OpenAI
export OPENAI_API_KEY=sk-...
agent-evals test ./agents/ --provider openai --model gpt-4o

# OpenAI-compatible (Ollama, vLLM, etc.)
agent-evals test ./agents/ \
    --provider openai-compatible \
    --base-url http://localhost:11434/v1 \
    --model llama3.3:70b \
    --api-key-env OLLAMA_API_KEY
```

## CLI Reference

### Shared Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--ci` | `false` | CI mode: JSON output, no pager, exit 1 on failure |
| `--format` | `terminal` | Output format: `terminal`, `json`, `markdown` |
| `--config` | auto-discover | Path to `agent-evals.yaml` |
| `-o, --output` | stdout | Write report to file |
| `--no-pager` | `false` | Disable automatic paging |

### Test-Only Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--provider` | `anthropic` | LLM provider |
| `--model` | provider default | Model for probes |
| `--base-url` | | Base URL for openai-compatible provider |
| `--api-key-env` | | Environment variable name for API key |
| `--probe-budget` | `500` | Maximum API calls for live probes |
| `--stochastic-runs` | `5` | Repeated runs per probe at T=0.7 |
| `--concurrency` | `3` | Maximum concurrent API calls |
| `--transcript` | | Write full probe Q&A to file (markdown) |

## CI Integration

```yaml
# .github/workflows/agent-evals.yaml
name: Agent Evals
on: [push, pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
      - run: go install github.com/thinkwright/agent-evals/cmd/agent-evals@latest
      - run: agent-evals check ./agents/ --ci
```

The `--ci` flag sets JSON output by default, disables the pager, and returns exit code 1 when scores fall below configured thresholds. For live probes in CI, set the appropriate API key as a repository secret and add `agent-evals test ./agents/ --ci --provider anthropic` as an additional step.

## Output Formats

Terminal output uses ANSI colors and pages through `less` when stdout is a TTY. JSON output is structured for CI pipelines and programmatic consumption. Markdown output is formatted for PR comments and report generation.

```sh
# Terminal (default, with pager)
agent-evals check ./agents/

# JSON for CI
agent-evals check ./agents/ --format json

# Markdown report to file
agent-evals test ./agents/ --format markdown -o report.md

# Full probe transcript
agent-evals test ./agents/ --transcript transcript.md
```

## License

MIT
