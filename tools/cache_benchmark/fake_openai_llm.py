#!/usr/bin/env python3
"""Deterministic OpenAI-compatible fake LLM for cache benchmark tests.

The server implements a small subset of /v1/chat/completions plus metrics
endpoints. It never calls a real LLM, making benchmark tests safe for CI and
local development without provider credentials.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import re
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any


STATE: dict[str, Any] = {
    "upstream_call_count": 0,
    "request_counts": {},
    "last_requests": [],
    "started_at": time.time(),
}


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def request_hash(body: dict[str, Any]) -> str:
    cache_relevant = {k: v for k, v in body.items() if k not in {"stream_options"}}
    return hashlib.sha256(canonical_json(cache_relevant).encode("utf-8")).hexdigest()


def estimate_tokens(value: Any) -> int:
    text = canonical_json(value)
    pieces = re.findall(r"[\w\u4e00-\u9fff]+|[^\s]", text, flags=re.UNICODE)
    return max(1, int(math.ceil(len(pieces) * 0.72 + len(text) / 48)))


def response_text(model: str, digest: str) -> str:
    return (
        "Deterministic cache benchmark response "
        f"for model {model} and request {digest[:12]}."
    )


def build_usage(body: dict[str, Any], digest: str) -> dict[str, int]:
    prompt_source = {
        "model": body.get("model"),
        "messages": body.get("messages", []),
        "tools": body.get("tools", []),
        "tool_choice": body.get("tool_choice"),
        "response_format": body.get("response_format"),
        "temperature": body.get("temperature"),
        "top_p": body.get("top_p"),
        "seed": body.get("seed"),
    }
    prompt_tokens = estimate_tokens(prompt_source)
    max_tokens = int(body.get("max_tokens") or 64)
    deterministic_completion = 16 + (int(digest[:2], 16) % 29)
    completion_tokens = max(1, min(max_tokens, deterministic_completion))
    return {
        "prompt_tokens": prompt_tokens,
        "completion_tokens": completion_tokens,
        "total_tokens": prompt_tokens + completion_tokens,
    }


def build_message(body: dict[str, Any], digest: str) -> dict[str, Any]:
    model = str(body.get("model") or "sub2api-cache-bench")
    tools = body.get("tools") or []
    tool_choice = body.get("tool_choice")
    if tools and tool_choice not in (None, "none"):
        first = tools[0].get("function", {}) if isinstance(tools[0], dict) else {}
        name = first.get("name") or "benchmark_tool"
        return {
            "role": "assistant",
            "content": None,
            "tool_calls": [
                {
                    "id": f"call_{digest[:10]}",
                    "type": "function",
                    "function": {
                        "name": name,
                        "arguments": json.dumps(
                            {"benchmark_request_hash": digest[:16]},
                            separators=(",", ":"),
                        ),
                    },
                }
            ],
        }
    return {"role": "assistant", "content": response_text(model, digest)}


def build_response(body: dict[str, Any]) -> dict[str, Any]:
    digest = request_hash(body)
    model = str(body.get("model") or "sub2api-cache-bench")
    usage = build_usage(body, digest)
    message = build_message(body, digest)
    finish_reason = "tool_calls" if message.get("tool_calls") else "stop"
    return {
        "id": f"chatcmpl_fake_{digest[:24]}",
        "object": "chat.completion",
        "created": 1700000000 + (int(digest[:4], 16) % 100000),
        "model": model,
        "choices": [
            {
                "index": 0,
                "message": message,
                "finish_reason": finish_reason,
            }
        ],
        "usage": usage,
        "system_fingerprint": "fake-openai-cache-benchmark-v1",
    }


def record_request(body: dict[str, Any]) -> str:
    digest = request_hash(body)
    STATE["upstream_call_count"] += 1
    counts = STATE["request_counts"]
    counts[digest] = int(counts.get(digest, 0)) + 1
    STATE["last_requests"].append(
        {
            "hash": digest,
            "model": body.get("model"),
            "stream": bool(body.get("stream")),
            "at": time.time(),
        }
    )
    STATE["last_requests"] = STATE["last_requests"][-25:]
    return digest


class Handler(BaseHTTPRequestHandler):
    server_version = "FakeOpenAI/1.0"

    def log_message(self, fmt: str, *args: Any) -> None:
        if getattr(self.server, "quiet", False):
            return
        super().log_message(fmt, *args)

    def _write_json(self, status: int, payload: Any, extra_headers: dict[str, str] | None = None) -> None:
        data = json.dumps(payload, ensure_ascii=False, sort_keys=True).encode("utf-8")
        self.send_response(status)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(data)))
        for key, value in (extra_headers or {}).items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(data)

    def _read_body(self) -> dict[str, Any]:
        length = int(self.headers.get("content-length") or "0")
        raw = self.rfile.read(length) if length else b"{}"
        if not raw:
            return {}
        return json.loads(raw.decode("utf-8"))

    def do_GET(self) -> None:
        if self.path in {"/health", "/v1/health"}:
            self._write_json(200, {"ok": True})
            return
        if self.path in {"/metrics", "/v1/metrics"}:
            self._write_json(200, STATE)
            return
        self._write_json(404, {"error": {"message": f"unknown path {self.path}"}})

    def do_POST(self) -> None:
        if self.path in {"/reset", "/v1/reset"}:
            STATE["upstream_call_count"] = 0
            STATE["request_counts"] = {}
            STATE["last_requests"] = []
            STATE["started_at"] = time.time()
            self._write_json(200, {"ok": True})
            return

        if not self.path.endswith("/chat/completions"):
            self._write_json(404, {"error": {"message": f"unknown path {self.path}"}})
            return

        try:
            body = self._read_body()
        except Exception as exc:  # pragma: no cover - defensive server path
            self._write_json(400, {"error": {"message": str(exc)}})
            return

        digest = record_request(body)
        response = build_response(body)
        headers = {
            "x-fake-request-hash": digest,
            "x-fake-upstream-call-count": str(STATE["upstream_call_count"]),
        }
        if bool(body.get("stream")):
            self._write_stream(response, headers)
            return
        self._write_json(200, response, headers)

    def _write_stream(self, response: dict[str, Any], headers: dict[str, str]) -> None:
        self.send_response(200)
        self.send_header("content-type", "text/event-stream")
        self.send_header("cache-control", "no-cache")
        for key, value in headers.items():
            self.send_header(key, value)
        self.end_headers()

        choice = response["choices"][0]
        message = choice["message"]
        if message.get("tool_calls"):
            delta = {"tool_calls": message["tool_calls"]}
        else:
            delta = {"content": message.get("content") or ""}
        chunk = {
            "id": response["id"],
            "object": "chat.completion.chunk",
            "created": response["created"],
            "model": response["model"],
            "choices": [{"index": 0, "delta": delta, "finish_reason": None}],
        }
        final = {
            "id": response["id"],
            "object": "chat.completion.chunk",
            "created": response["created"],
            "model": response["model"],
            "choices": [{"index": 0, "delta": {}, "finish_reason": choice["finish_reason"]}],
            "usage": response["usage"],
        }
        for event in (chunk, final):
            self.wfile.write(b"data: ")
            self.wfile.write(json.dumps(event, ensure_ascii=False, sort_keys=True).encode("utf-8"))
            self.wfile.write(b"\n\n")
            self.wfile.flush()
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18081)
    parser.add_argument("--quiet", action="store_true")
    args = parser.parse_args(argv)

    server = ThreadingHTTPServer((args.host, args.port), Handler)
    server.quiet = args.quiet  # type: ignore[attr-defined]
    if not args.quiet:
        print(f"fake OpenAI-compatible LLM listening on http://{args.host}:{args.port}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        return 0
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
