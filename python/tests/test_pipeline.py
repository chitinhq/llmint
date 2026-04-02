"""Tests for the LLMint Python SDK."""
from __future__ import annotations

import pytest

from llmint.types import ContentBlock, Response, Savings, Stats, Usage


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def check_lib():
    """Skip the test if the shared library cannot be found."""
    try:
        from llmint._ffi import _find_lib
        if _find_lib() is None:
            pytest.skip("LLMint shared library not found — build cabi first")
    except Exception as e:
        pytest.skip(f"FFI not available: {e}")


# ---------------------------------------------------------------------------
# Pure-Python type tests — no shared library needed
# ---------------------------------------------------------------------------

def test_types_response_from_dict():
    d = {
        "content": [{"type": "text", "text": "hello"}],
        "usage": {
            "InputTokens": 10,
            "OutputTokens": 5,
            "CacheReadTokens": 0,
            "CacheWriteTokens": 0,
            "Cost": 0.001,
        },
        "model": "mock-model",
        "cache_status": "miss",
        "savings": [{"TokensSaved": 15, "CostSaved": 0.0005, "Technique": "dedup"}],
    }
    resp = Response.from_dict(d)
    assert resp.model == "mock-model"
    assert resp.cache_status == "miss"
    assert len(resp.content) == 1
    assert resp.content[0].text == "hello"
    assert resp.usage.input_tokens == 10
    assert resp.usage.output_tokens == 5
    assert resp.usage.cost == pytest.approx(0.001)
    assert len(resp.savings) == 1
    assert resp.savings[0].technique == "dedup"
    assert resp.savings[0].tokens_saved == 15

    stats = Stats.from_dict({"total_calls": 3, "total_tokens_saved": 42, "total_cost_saved": 0.01})
    assert stats.total_calls == 3
    assert stats.total_tokens_saved == 42


# ---------------------------------------------------------------------------
# Integration tests — require shared library
# ---------------------------------------------------------------------------

def test_pipeline_create_and_complete(check_lib):
    from llmint import Pipeline

    with Pipeline(provider="mock") as p:
        resp = p.complete(
            model="mock-model",
            messages=[{"role": "user", "content": "Hello, world!"}],
        )
    assert resp.model != ""
    assert len(resp.content) > 0
    assert resp.content[0].text != ""
    assert resp.usage.input_tokens > 0


def test_pipeline_stats(check_lib):
    from llmint import Pipeline
    from llmint import middleware

    with Pipeline(middleware.account(), provider="mock") as p:
        p.complete(
            model="mock-model",
            messages=[{"role": "user", "content": "count me"}],
        )
        stats = p.stats()

    assert stats.total_calls == 1


def test_pipeline_dedup(check_lib):
    from llmint import Pipeline
    from llmint import middleware

    with Pipeline(middleware.dedup(), provider="mock") as p:
        req = dict(
            model="mock-model",
            messages=[{"role": "user", "content": "same request every time"}],
        )
        r1 = p.complete(**req)
        r2 = p.complete(**req)

    # Second call should be served from dedup cache.
    assert r2.cache_status == "hit"
    assert any(s.technique == "dedup" for s in r2.savings)
