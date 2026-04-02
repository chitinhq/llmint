#!/usr/bin/env python3
"""basic.py — LLMint Python SDK usage example.

Demonstrates a dedup + account pipeline using the mock provider.
The same request is sent twice; the second call shows a cache hit.

Usage:
    cd examples/python
    python3 basic.py
"""
from llmint import Pipeline
from llmint import middleware


def main() -> None:
    # Build pipeline: dedup → account → mock provider.
    with Pipeline(
        middleware.dedup(ttl=600),
        middleware.account(sink="stdout"),
        provider="mock",
    ) as pipeline:

        request = dict(
            model="mock-model",
            messages=[
                {"role": "user", "content": "What is the answer to life, the universe, and everything?"}
            ],
            max_tokens=256,
        )

        # First call — cache miss, hits the mock provider.
        resp1 = pipeline.complete(**request)
        print_result("Call 1", resp1)

        # Second call — identical request, served from dedup cache.
        resp2 = pipeline.complete(**request)
        print_result("Call 2", resp2)

        # Show aggregated stats.
        stats = pipeline.stats()
        print(f"\nStats: calls={stats.total_calls} "
              f"tokens_saved={stats.total_tokens_saved} "
              f"cost_saved=${stats.total_cost_saved:.6f}")


def print_result(label: str, resp) -> None:
    text = resp.content[0].text if resp.content else ""
    print(f"[{label}] model={resp.model} cache={resp.cache_status} text={text!r}")
    print(f"        tokens in={resp.usage.input_tokens} out={resp.usage.output_tokens}")
    for s in resp.savings:
        print(f"        savings: {s.tokens_saved} tokens via {s.technique}")


if __name__ == "__main__":
    main()
