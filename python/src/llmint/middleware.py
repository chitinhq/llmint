"""Helper functions that build middleware config dicts for use in Pipeline."""
from __future__ import annotations

from typing import List


def dedup(ttl: int = 600) -> dict:
    """Deduplication middleware — caches identical requests."""
    return {"type": "dedup", "params": {"ttl": ttl}}


def cascade(models: List[str]) -> dict:
    """Cascade middleware — escalates to higher-capability models as needed."""
    return {"type": "cascade", "params": {"models": models}}


def prompt_cache(window: int = 300) -> dict:
    """Prompt-cache middleware — reuses cached prompt prefixes."""
    return {"type": "prompt_cache", "params": {"window": window}}


def batch(max_size: int = 10, max_wait: int = 300) -> dict:
    """Batch middleware — groups requests for throughput efficiency."""
    return {"type": "batch", "params": {"max_size": max_size, "max_wait": max_wait}}


def account(sink: str = "stdout") -> dict:
    """Accounting middleware — records token usage and cost."""
    return {"type": "account", "params": {"sink": sink}}
