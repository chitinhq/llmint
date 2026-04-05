# llmint

**LLM token economics for Go** — provider abstractions, middleware composition, cache-aware cost tracking.

## Install

```bash
go get github.com/chitinhq/llmint
```

## Core concepts

| Type | Purpose |
|------|---------|
| `Provider` | Interface every LLM backend implements (`Complete`, `Name`, `Models`) |
| `Middleware` | `func(Provider) Provider` — wrap providers with cross-cutting concerns |
| `Request` / `Response` | Canonical input/output, provider-agnostic |
| `Usage` | Raw token counts + `ComputeCost(ModelInfo)` |
| `Savings` | Per-technique savings; `TotalSavings([]Savings)` aggregates |
| `CacheStatus` | `CacheMiss` / `CacheHit` / `CachePartial` |

## Quick start

```go
import (
    "context"
    "github.com/chitinhq/llmint"
    "github.com/chitinhq/llmint/provider/mock"
)

p := mock.New("claude-3-5-sonnet-20241022", "Hello, world!")
resp, err := p.Complete(context.Background(), &llmint.Request{
    Model:    "claude-3-5-sonnet-20241022",
    Messages: []llmint.Message{{Role: "user", Content: "Hi"}},
})
```

## Middleware

```go
// Chain applies middleware left-to-right (first = outermost, like net/http).
wrapped := llmint.Chain(logging, rateLimit, cache)(baseProvider)
```

## License

Apache-2.0
