# LLMint

**Go library for tracking and controlling what your LLM calls cost.**

LLMint wraps calls to LLM providers (Anthropic, OpenAI, others) in a
middleware chain so you can see, in USD, what each request cost and
how much you saved from caching, deduping, or routing to a cheaper
model.

It is a pure Go library. No binaries, no services, zero external
dependencies. You import it and wrap your provider client.

LLMint is the **metabolism** of the Chitin Platform — the layer that
decides how tokens (energy) get spent.

## What problem does this solve?

Agent fleets burn tokens. Without accounting you get a monthly bill
and no idea which agent, which prompt, or which model tier caused it.
LLMint gives you:

- Per-request USD cost, derived from each model's pricing table.
- A pluggable sink so cost data lands wherever you want (stdout, DB,
  Prometheus).
- Composable middleware to reduce the bill: dedup identical requests,
  batch small ones, cache long system prompts, cascade from cheap to
  expensive models only when confidence is low.

## Try it

```bash
go get github.com/chitinhq/llmint
```

```go
import (
    "context"
    "github.com/chitinhq/llmint"
    "github.com/chitinhq/llmint/middleware/account"
    "github.com/chitinhq/llmint/middleware/dedup"
    "github.com/chitinhq/llmint/provider/anthropic"
)

// Wrap a provider with dedup (cache identical requests) + account
// (record cost). Middleware composes left-to-right, outermost first.
base := anthropic.New("sk-ant-...")
p := llmint.Chain(account.New(sink), dedup.New())(base)

resp, err := p.Complete(context.Background(), &llmint.Request{
    Model:    "claude-3-5-sonnet-20241022",
    Messages: []llmint.Message{{Role: "user", Content: "Hi"}},
})
// resp.Usage.ComputeCost(modelInfo) returns the USD cost.
```

### Middleware you can stack

| Package                  | What it does                                            |
|--------------------------|---------------------------------------------------------|
| `middleware/account`     | Records tokens, cost, and duration to a pluggable sink  |
| `middleware/dedup`       | Caches responses by request hash (identical-in, cached) |
| `middleware/batch`       | Queues requests, flushes on size or time                |
| `middleware/promptcache` | Marks system prompts for provider-side prompt caching   |
| `middleware/distill`     | Replaces long system prompts with shorter equivalents   |
| `middleware/cascade`     | Tries cheap models first, escalates on low confidence   |

### Providers

`provider/anthropic`, `provider/openai`, `provider/mock` (for tests).

## Where next

- [`cabi/`](./cabi/) — C FFI bindings if you want to call LLMint from
  Python, Ruby, or anything else that speaks C.
- [`python/`](./python/) — separate Python package for cost analytics
  on what LLMint's sink writes out.
- [Chitin Platform overview](https://github.com/chitinhq/workspace) —
  LLMint works standalone, but it is also the cost layer for
  Chitin-governed agent fleets.

## Development

```bash
go build ./...
go test ./...
```

## License

MIT
