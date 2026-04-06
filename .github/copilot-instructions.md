# LLMint — Copilot Instructions

> Copilot acts as **Tier C — Execution Workforce** in this repository.
> Implement well-specified issues, open draft PRs, never merge or approve.

## Project Overview

**LLMint** is the token economics middleware for the Chitin platform. It tracks LLM costs, enforces budget rules, and enables model cascading. Octi Pulpo imports LLMint for dispatch budget gating.

**Core principle**: Every token has a cost. LLMint makes that cost visible and enforceable.

## Tech Stack

- **Languages**: Go + Python
- **Go module**: `github.com/chitinhq/llmint`
- **Version**: v0.1.0 (semver — breaking changes require major bump)

## Repository Structure

```
cmd/llmint/           # Go binary entrypoint
internal/
├── tracker/          # Cost tracking per request/session
├── budget/           # Budget rules and enforcement
├── cascade/          # Model cascading logic (Haiku → Sonnet → Opus)
└── config/           # Configuration
llmint/               # Python package (analytics, cost reporting)
```

## Build & Test

```bash
# Go
go build ./...
go test ./...
golangci-lint run

# Python
python -m pytest tests/
```

## Coding Standards

- Follow Go conventions (`gofmt`, `go vet`)
- Python: follow existing style, type hints preferred
- Error handling: always check and wrap errors with context
- This is a published module — API stability matters, semver is enforced

## Governance Rules

### DENY
- `git push` to main — always use feature branches
- `git force-push` — never rewrite shared history
- Write to `.env`, SSH keys, credentials
- Write or delete `.claude/` files
- Execute `rm -rf` or destructive shell commands

### ALWAYS
- Create feature branches: `agent/<type>/issue-<N>`
- Run `go build ./... && go test ./...` before creating PRs
- Include governance report in PR body
- Link PRs to issues (`Closes #N`)

## Three-Tier Model

- **Tier A — Architect** (Claude Opus): Sprint planning, architecture, risk
- **Tier B — Senior** (@claude on GitHub): Complex implementation, code review
- **Tier C — Execution** (Copilot): Implement specified issues, open draft PRs

### PR Rules

- **NEVER merge PRs** — only Tier B or humans merge
- **NEVER approve PRs** — post first-pass review comments only
- Max 300 lines changed per PR (soft limit)
- Always open as **draft PR** first
- If ambiguous, label `needs-spec` and stop

## Critical Areas

- `internal/budget/` — budget enforcement, cost impact on all dispatch
- `internal/cascade/` — model selection, affects token spend
- Public Go API surface — semver, don't break consumers

## Branch Naming

```
agent/feat/issue-<N>
agent/fix/issue-<N>
agent/refactor/issue-<N>
agent/test/issue-<N>
agent/docs/issue-<N>
```

## Autonomy Directive

- **NEVER pause to ask for clarification** — make your best judgment
- If the issue is ambiguous, label it `needs-spec` and stop
- Default to the **safest option** in every ambiguous situation
