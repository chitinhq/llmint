## Agent Identity

At session start, if you see `[AgentGuard] No agent identity set`, ask the user:
1. **Role**: developer / reviewer / ops / security / planner
2. **Driver**: human / claude-code / copilot / ci

Then run: `scripts/write-persona.sh <driver> <role>`

## Project

LLMint is the token economics middleware for the Chitin platform. It tracks LLM costs, enforces budget rules, and enables model cascading (Haiku → Sonnet → Opus).

**Module**: `github.com/chitinhq/llmint`
**Languages**: Go + Python
**Version**: v0.1.0 (semver enforced)

## Key Directories

- `cmd/llmint/` — binary entrypoint
- `internal/tracker/` — cost tracking per request/session
- `internal/budget/` — budget rules and enforcement (cost impact, careful)
- `internal/cascade/` — model cascading logic
- `llmint/` — Python package for analytics

## Build

```bash
go build ./...
go test ./...
golangci-lint run
python -m pytest tests/
```

## Assembly Line

LLMint is imported by Octi Pulpo for dispatch budget gating. Every agent task has a cost budget in its work contract — LLMint enforces it.
