"""Core dataclasses for LLMint responses and statistics."""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import List, Optional


@dataclass
class Usage:
    input_tokens: int = 0
    output_tokens: int = 0
    cache_read_tokens: int = 0
    cache_write_tokens: int = 0
    cost: float = 0.0


@dataclass
class Savings:
    tokens_saved: int = 0
    cost_saved: float = 0.0
    technique: str = ""


@dataclass
class ContentBlock:
    type: str = "text"
    text: str = ""


@dataclass
class Response:
    content: List[ContentBlock] = field(default_factory=list)
    usage: Usage = field(default_factory=Usage)
    model: str = ""
    cache_status: str = "miss"
    savings: List[Savings] = field(default_factory=list)

    @classmethod
    def from_dict(cls, d: dict) -> "Response":
        content = [
            ContentBlock(
                type=b.get("Type", b.get("type", "text")),
                text=b.get("Text", b.get("text", "")),
            )
            for b in d.get("content") or []
        ]
        u = d.get("usage") or {}
        usage = Usage(
            input_tokens=u.get("InputTokens", u.get("input_tokens", 0)),
            output_tokens=u.get("OutputTokens", u.get("output_tokens", 0)),
            cache_read_tokens=u.get("CacheReadTokens", u.get("cache_read_tokens", 0)),
            cache_write_tokens=u.get("CacheWriteTokens", u.get("cache_write_tokens", 0)),
            cost=u.get("Cost", u.get("cost", 0.0)),
        )
        savings = [
            Savings(
                tokens_saved=s.get("TokensSaved", s.get("tokens_saved", 0)),
                cost_saved=s.get("CostSaved", s.get("cost_saved", 0.0)),
                technique=s.get("Technique", s.get("technique", "")),
            )
            for s in d.get("savings") or []
        ]
        return cls(
            content=content,
            usage=usage,
            model=d.get("model", ""),
            cache_status=d.get("cache_status", "miss"),
            savings=savings,
        )


@dataclass
class Stats:
    total_calls: int = 0
    total_tokens_saved: int = 0
    total_cost_saved: float = 0.0

    @classmethod
    def from_dict(cls, d: dict) -> "Stats":
        return cls(
            total_calls=d.get("total_calls", 0),
            total_tokens_saved=d.get("total_tokens_saved", 0),
            total_cost_saved=d.get("total_cost_saved", 0.0),
        )
