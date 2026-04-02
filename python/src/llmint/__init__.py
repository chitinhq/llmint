"""LLMint Python SDK — token economics library with Go-backed pipeline."""
from __future__ import annotations

from typing import Dict, List, Optional

from . import _ffi
from .types import Response, Stats


class Pipeline:
    """A configured LLM pipeline backed by the Go shared library.

    Usage::

        with Pipeline(middleware.dedup(), middleware.account(), provider="mock") as p:
            resp = p.complete(model="mock-model", messages=[{"role": "user", "content": "hi"}])
            print(resp.content[0].text)
    """

    def __init__(
        self,
        *middleware,
        provider: str = "mock",
        api_key: str = "",
    ) -> None:
        config = {
            "provider": provider,
            "api_key": api_key,
            "middleware": list(middleware),
        }
        self._id = _ffi.create_pipeline(config)

    def complete(
        self,
        model: str = "",
        messages: Optional[List[Dict]] = None,
        system: str = "",
        max_tokens: int = 1024,
        metadata: Optional[Dict[str, str]] = None,
    ) -> Response:
        """Send a completion request through the pipeline.

        Args:
            model: Model identifier (e.g. ``"claude-3-haiku-20240307"``).
            messages: List of ``{"role": ..., "content": ...}`` dicts.
            system: System prompt string.
            max_tokens: Maximum tokens to generate.
            metadata: Optional key-value metadata (tracing hints, not hashed).

        Returns:
            A :class:`~llmint.types.Response` with content, usage, and savings.
        """
        # Normalise messages to match Go's Message struct field names.
        normalised = []
        for m in messages or []:
            normalised.append({
                "Role": m.get("role", m.get("Role", "user")),
                "Content": m.get("content", m.get("Content", "")),
            })

        request = {
            "Model": model,
            "Messages": normalised,
            "System": system,
            "MaxTokens": max_tokens,
            "Metadata": metadata or {},
        }
        result = _ffi.complete(self._id, request)
        if "error" in result:
            raise RuntimeError(f"LLMint error: {result['error']}")
        return Response.from_dict(result)

    def stats(self) -> Stats:
        """Return aggregated stats for this pipeline."""
        result = _ffi.get_stats(self._id)
        if "error" in result:
            raise RuntimeError(f"LLMint error: {result['error']}")
        return Stats.from_dict(result)

    def close(self) -> None:
        """Release the pipeline from the Go registry."""
        if self._id > 0:
            _ffi.free_pipeline(self._id)
            self._id = -1

    def __enter__(self) -> "Pipeline":
        return self

    def __exit__(self, *_) -> None:
        self.close()
