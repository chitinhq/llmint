"""ctypes FFI bindings to the LLMint shared library."""
from __future__ import annotations

import ctypes
import json
import os
import sys
from pathlib import Path
from typing import Optional


def _find_lib() -> Optional[str]:
    """Search for the LLMint shared library in likely locations."""
    # Determine platform-specific filename patterns.
    if sys.platform.startswith("linux"):
        candidates = ["libllmint-linux-amd64.so", "libllmint.so"]
    elif sys.platform == "darwin":
        candidates = [
            "libllmint-darwin-arm64.dylib",
            "libllmint-darwin-amd64.dylib",
            "libllmint.dylib",
        ]
    else:
        candidates = []

    # Directories to search.
    pkg_dir = Path(__file__).parent
    cabi_dir = pkg_dir.parent.parent.parent / "cabi"  # repo layout: python/src/llmint → cabi
    search_dirs = [pkg_dir, cabi_dir, Path.cwd(), Path.cwd() / "cabi"]

    for dirname in search_dirs:
        for name in candidates:
            p = dirname / name
            if p.exists():
                return str(p)

    return None


_lib: Optional[ctypes.CDLL] = None


def _get_lib() -> ctypes.CDLL:
    global _lib
    if _lib is not None:
        return _lib

    path = _find_lib()
    if path is None:
        raise FileNotFoundError(
            "LLMint shared library not found. "
            "Build it with: cd cabi && make linux"
        )

    lib = ctypes.CDLL(path)

    # LLMintCreatePipeline(configJSON *C.char) C.longlong
    lib.LLMintCreatePipeline.argtypes = [ctypes.c_char_p]
    lib.LLMintCreatePipeline.restype = ctypes.c_longlong

    # LLMintComplete(pipelineID C.longlong, requestJSON *C.char) *C.char
    lib.LLMintComplete.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
    lib.LLMintComplete.restype = ctypes.c_char_p

    # LLMintGetStats(pipelineID C.longlong) *C.char
    lib.LLMintGetStats.argtypes = [ctypes.c_longlong]
    lib.LLMintGetStats.restype = ctypes.c_char_p

    # LLMintFreePipeline(pipelineID C.longlong)
    lib.LLMintFreePipeline.argtypes = [ctypes.c_longlong]
    lib.LLMintFreePipeline.restype = None

    # LLMintFreeString(s *C.char)
    lib.LLMintFreeString.argtypes = [ctypes.c_char_p]
    lib.LLMintFreeString.restype = None

    _lib = lib
    return lib


def create_pipeline(config: dict) -> int:
    """Create a pipeline from a config dict. Returns pipeline ID (>0) or raises."""
    lib = _get_lib()
    payload = json.dumps(config).encode("utf-8")
    pid = lib.LLMintCreatePipeline(payload)
    if pid < 0:
        raise ValueError("Failed to create pipeline — check config JSON")
    return int(pid)


def complete(pipeline_id: int, request: dict) -> dict:
    """Call Complete on the pipeline. Returns response dict (may contain 'error')."""
    lib = _get_lib()
    payload = json.dumps(request).encode("utf-8")
    raw = lib.LLMintComplete(ctypes.c_longlong(pipeline_id), payload)
    if raw is None:
        return {"error": "null response from LLMintComplete"}
    result = json.loads(raw)
    return result


def get_stats(pipeline_id: int) -> dict:
    """Fetch stats for a pipeline. Returns stats dict."""
    lib = _get_lib()
    raw = lib.LLMintGetStats(ctypes.c_longlong(pipeline_id))
    if raw is None:
        return {"error": "null response from LLMintGetStats"}
    return json.loads(raw)


def free_pipeline(pipeline_id: int) -> None:
    """Release a pipeline from the Go-side registry."""
    lib = _get_lib()
    lib.LLMintFreePipeline(ctypes.c_longlong(pipeline_id))
