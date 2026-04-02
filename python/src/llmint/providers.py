"""Provider config helpers for use in Pipeline."""
from __future__ import annotations


def Anthropic(api_key: str) -> dict:
    """Return a provider config dict for the Anthropic backend."""
    return {"provider": "anthropic", "api_key": api_key}
