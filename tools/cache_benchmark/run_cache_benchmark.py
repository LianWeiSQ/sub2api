#!/usr/bin/env python3
"""Run Sub2API/LiteLLM gateway cache benchmark samples.

The default mode is offline-safe: it starts the fake OpenAI-compatible LLM and
uses it as the baseline endpoint. To benchmark Sub2API, pass --baseline-url to
the Sub2API OpenAI-compatible chat completions endpoint after configuring a test
account that points at the fake LLM.
"""

from __future__ import annotations

import argparse
import csv
import json
import os
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any
from urllib.parse import urlparse


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_FIXTURES = ROOT / "tools" / "cache_benchmark" / "fixtures.json"
DEFAULT_OUTPUT_DIR = ROOT / "tools" / "cache_benchmark" / "results"
DEFAULT_FAKE_BASE_URL = "http://127.0.0.1:18081"
DEFAULT_LITELLM_URL = "http://127.0.0.1:4000/v1/chat/completions"
CACHE_STATUS_HEADERS = [
    "x-sub2api-cache-status",
    "x-litellm-cache-hit",
    "x-litellm-cache-status",
    "cf-aig-cache-status",
    "x-cache",
]


class BenchmarkError(RuntimeError):
    pass


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def load_fixtures(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        data = json.load(handle)
    samples = data.get("samples")
    if not isinstance(samples, list) or len(samples) != 10:
        raise BenchmarkError(f"{path} must contain exactly 10 samples")
    seen: set[str] = set()
    for sample in samples:
        sample_id = sample.get("id")
        body = sample.get("body")
        if not sample_id or sample_id in seen:
            raise BenchmarkError(f"duplicate or missing sample id: {sample_id}")
        seen.add(sample_id)
        if not isinstance(body, dict) or "messages" not in body or "model" not in body:
            raise BenchmarkError(f"sample {sample_id} must include OpenAI chat body")
    return data


def http_json(method: str, url: str, payload: Any | None = None, timeout: float = 10) -> tuple[int, dict[str, str], Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    headers = {"content-type": "application/json"}
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        raw = resp.read()
        body = json.loads(raw.decode("utf-8")) if raw else None
        return resp.status, {k.lower(): v for k, v in resp.headers.items()}, body


def wait_for_health(base_url: str, timeout: float) -> None:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            status, _, body = http_json("GET", f"{base_url.rstrip('/')}/health", timeout=1)
            if status == 200 and body.get("ok"):
                return
        except Exception as exc:  # pragma: no cover - timing dependent
            last_error = exc
        time.sleep(0.1)
    raise BenchmarkError(f"fake LLM did not become healthy: {last_error}")


def start_fake_server(fake_base_url: str, quiet: bool) -> subprocess.Popen[bytes]:
    from urllib.parse import urlparse

    parsed = urlparse(fake_base_url)
    host = parsed.hostname or "127.0.0.1"
    port = parsed.port or 18081
    script = ROOT / "tools" / "cache_benchmark" / "fake_openai_llm.py"
    cmd = [sys.executable, str(script), "--host", host, "--port", str(port)]
    if quiet:
        cmd.append("--quiet")
    proc = subprocess.Popen(cmd)
    wait_for_health(fake_base_url, timeout=10)
    return proc


def stop_process(proc: subprocess.Popen[bytes] | None) -> None:
    if proc is None or proc.poll() is not None:
        return
    proc.send_signal(signal.SIGTERM)
    try:
        proc.wait(timeout=3)
    except subprocess.TimeoutExpired:  # pragma: no cover - defensive cleanup
        proc.kill()
        proc.wait(timeout=3)


def reset_fake(fake_base_url: str, timeout: float) -> None:
    http_json("POST", f"{fake_base_url.rstrip('/')}/reset", payload={}, timeout=timeout)


def fake_metrics(fake_base_url: str, timeout: float) -> dict[str, Any]:
    _, _, body = http_json("GET", f"{fake_base_url.rstrip('/')}/metrics", timeout=timeout)
    return body


def parse_usage(payload: dict[str, Any]) -> dict[str, int]:
    usage = payload.get("usage") if isinstance(payload, dict) else None
    if not isinstance(usage, dict):
        return {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
    prompt = int(usage.get("prompt_tokens") or usage.get("input_tokens") or 0)
    completion = int(usage.get("completion_tokens") or usage.get("output_tokens") or 0)
    total = int(usage.get("total_tokens") or prompt + completion)
    return {"prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": total}


def read_sse_response(resp: Any, start: float) -> tuple[dict[str, Any], float]:
    usage: dict[str, Any] = {}
    content_parts: list[str] = []
    chunks = 0
    first_token_ms = 0.0
    for raw_line in resp:
        line = raw_line.decode("utf-8", errors="replace").strip()
        if not line.startswith("data:"):
            continue
        data = line[len("data:") :].strip()
        if data == "[DONE]":
            break
        if not data:
            continue
        chunks += 1
        if first_token_ms == 0.0:
            first_token_ms = (time.perf_counter() - start) * 1000
        event = json.loads(data)
        if isinstance(event.get("usage"), dict):
            usage = event["usage"]
        choices = event.get("choices") or []
        if choices:
            delta = choices[0].get("delta") or {}
            if isinstance(delta.get("content"), str):
                content_parts.append(delta["content"])
    return (
        {
            "object": "chat.completion.stream_aggregate",
            "choices": [{"message": {"role": "assistant", "content": "".join(content_parts)}}],
            "usage": usage,
            "stream_chunks": chunks,
        },
        first_token_ms,
    )


def call_chat_completion(url: str, api_key: str, body: dict[str, Any], timeout: float) -> tuple[int, dict[str, str], dict[str, Any], float, float]:
    data = json.dumps(body, ensure_ascii=False).encode("utf-8")
    headers = {
        "authorization": f"Bearer {api_key}",
        "content-type": "application/json",
        "accept": "text/event-stream" if body.get("stream") else "application/json",
    }
    req = urllib.request.Request(url, data=data, headers=headers, method="POST")
    start = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            first_token_ms = (time.perf_counter() - start) * 1000
            if body.get("stream"):
                payload, first_token_ms = read_sse_response(resp, start)
            else:
                raw = resp.read()
                payload = json.loads(raw.decode("utf-8")) if raw else {}
            duration_ms = (time.perf_counter() - start) * 1000
            return resp.status, {k.lower(): v for k, v in resp.headers.items()}, payload, duration_ms, first_token_ms
    except urllib.error.HTTPError as exc:
        raw = exc.read()
        duration_ms = (time.perf_counter() - start) * 1000
        try:
            payload = json.loads(raw.decode("utf-8")) if raw else {}
        except json.JSONDecodeError:
            payload = {"error": raw.decode("utf-8", errors="replace")}
        return exc.code, {k.lower(): v for k, v in exc.headers.items()}, payload, duration_ms, duration_ms


def route_health_url(chat_url: str) -> str:
    parsed = urlparse(chat_url)
    if not parsed.scheme or not parsed.netloc:
        return chat_url
    return f"{parsed.scheme}://{parsed.netloc}/health"


def route_available(chat_url: str, timeout: float) -> bool:
    try:
        req = urllib.request.Request(route_health_url(chat_url), method="GET")
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return 200 <= resp.status < 500
    except urllib.error.HTTPError as exc:
        return exc.code < 500
    except Exception:
        return False


def header_cache_status(headers: dict[str, str]) -> str:
    for key in CACHE_STATUS_HEADERS:
        if key in headers:
            return f"{key}={headers[key]}"
    return ""


def infer_cache_status(route: str, status_code: int, upstream_delta: int, headers: dict[str, str], cacheable: bool) -> str:
    if status_code >= 400:
        return "error"
    if not cacheable:
        return "bypass"
    explicit = header_cache_status(headers).lower()
    if "true" in explicit or "hit" in explicit:
        return "hit"
    if upstream_delta == 0 and route != "baseline":
        return "hit"
    if upstream_delta > 0:
        return "miss"
    return "unknown"


def run_route(
    route_name: str,
    url: str,
    api_key: str,
    samples: list[dict[str, Any]],
    fake_base_url: str,
    passes: int,
    timeout: float,
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    reset_fake(fake_base_url, timeout=timeout)
    for pass_index in range(1, passes + 1):
        for sample in samples:
            body = json.loads(canonical_json(sample["body"]))
            before = fake_metrics(fake_base_url, timeout=timeout)["upstream_call_count"]
            error = ""
            try:
                status_code, headers, payload, duration_ms, first_token_ms = call_chat_completion(url, api_key, body, timeout=timeout)
            except Exception as exc:
                status_code, headers, payload, duration_ms, first_token_ms = 0, {}, {}, 0.0, 0.0
                error = str(exc)
            after = fake_metrics(fake_base_url, timeout=timeout)["upstream_call_count"]
            upstream_delta = max(0, int(after) - int(before))
            usage = parse_usage(payload)
            cache_status = infer_cache_status(
                route_name,
                status_code,
                upstream_delta,
                headers,
                bool(sample.get("cacheable", True)),
            )
            saved_input = usage["prompt_tokens"] if cache_status == "hit" else 0
            saved_output = usage["completion_tokens"] if cache_status == "hit" else 0
            rows.append(
                {
                    "route": route_name,
                    "sample_id": sample["id"],
                    "category": sample["category"],
                    "cacheable": bool(sample.get("cacheable", True)),
                    "pass_index": pass_index,
                    "stream": bool(body.get("stream")),
                    "status_code": status_code,
                    "cache_status": cache_status,
                    "cache_status_header": header_cache_status(headers),
                    "input_tokens": usage["prompt_tokens"],
                    "output_tokens": usage["completion_tokens"],
                    "total_tokens": usage["total_tokens"],
                    "cache_read_tokens": usage["prompt_tokens"] if cache_status == "hit" else 0,
                    "gateway_saved_input_tokens": saved_input,
                    "gateway_saved_output_tokens": saved_output,
                    "gateway_saved_tokens": saved_input + saved_output,
                    "duration_ms": round(duration_ms, 3),
                    "first_token_ms": round(first_token_ms, 3),
                    "upstream_call_count": after,
                    "upstream_delta": upstream_delta,
                    "error": error,
                }
            )
    return rows


def summarize(rows: list[dict[str, Any]], target_hit_rate: float) -> dict[str, Any]:
    summary: dict[str, Any] = {"target_hit_rate": target_hit_rate, "routes": {}}
    for route in sorted({row["route"] for row in rows}):
        route_rows = [row for row in rows if row["route"] == route]
        cacheable_rows = [
            row
            for row in route_rows
            if row["cacheable"] and row["status_code"] and row["status_code"] < 400
        ]
        hits = sum(1 for row in cacheable_rows if row["cache_status"] == "hit")
        misses = sum(1 for row in cacheable_rows if row["cache_status"] == "miss")
        hit_rate = hits / len(cacheable_rows) if cacheable_rows else 0.0
        summary["routes"][route] = {
            "rows": len(route_rows),
            "cacheable_rows": len(cacheable_rows),
            "hits": hits,
            "misses": misses,
            "hit_rate": round(hit_rate, 4),
            "target_met": hit_rate >= target_hit_rate if cacheable_rows else False,
            "gateway_saved_tokens": sum(int(row["gateway_saved_tokens"]) for row in route_rows),
            "gateway_saved_input_tokens": sum(int(row["gateway_saved_input_tokens"]) for row in route_rows),
            "gateway_saved_output_tokens": sum(int(row["gateway_saved_output_tokens"]) for row in route_rows),
            "upstream_calls": sum(int(row["upstream_delta"]) for row in route_rows),
            "avg_duration_ms": round(
                sum(float(row["duration_ms"]) for row in route_rows) / len(route_rows),
                3,
            )
            if route_rows
            else 0,
            "avg_first_token_ms": round(
                sum(float(row["first_token_ms"]) for row in route_rows) / len(route_rows),
                3,
            )
            if route_rows
            else 0,
        }
    return summary


def write_outputs(output_dir: Path, rows: list[dict[str, Any]], summary: dict[str, Any]) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    json_path = output_dir / "cache_benchmark_results.json"
    csv_path = output_dir / "cache_benchmark_results.csv"
    summary_path = output_dir / "cache_benchmark_summary.json"
    json_path.write_text(json.dumps(rows, ensure_ascii=False, indent=2), encoding="utf-8")
    summary_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    if rows:
        with csv_path.open("w", encoding="utf-8", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=list(rows[0].keys()))
            writer.writeheader()
            writer.writerows(rows)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fixtures", type=Path, default=DEFAULT_FIXTURES)
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT_DIR)
    parser.add_argument("--fake-base-url", default=os.environ.get("FAKE_OPENAI_BASE_URL", DEFAULT_FAKE_BASE_URL).rstrip("/"))
    parser.add_argument("--baseline-url", default=os.environ.get("SUB2API_BENCHMARK_URL"))
    parser.add_argument("--litellm-url", default=os.environ.get("LITELLM_BENCHMARK_URL", DEFAULT_LITELLM_URL))
    parser.add_argument("--baseline-api-key", default=os.environ.get("SUB2API_BENCHMARK_API_KEY", "bench-key"))
    parser.add_argument("--litellm-api-key", default=os.environ.get("LITELLM_BENCHMARK_API_KEY", "sk-bench-key"))
    parser.add_argument("--routes", default="baseline,litellm", help="Comma separated: baseline,litellm")
    parser.add_argument("--passes", type=int, default=10, help="Default 10 gives a 90%% exact-cache target after first miss.")
    parser.add_argument("--target-hit-rate", type=float, default=0.90)
    parser.add_argument("--timeout", type=float, default=20)
    parser.add_argument("--start-fake", action="store_true", help="Start the local fake OpenAI server for the run.")
    parser.add_argument("--quiet-fake", action="store_true")
    parser.add_argument("--skip-litellm-if-unavailable", action="store_true", default=True)
    parser.add_argument("--require-litellm", action="store_true", help="Fail if LiteLLM route is unavailable.")
    parser.add_argument("--dry-run", action="store_true", help="Validate fixtures and print planned routes only.")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    fixtures = load_fixtures(args.fixtures)
    samples = fixtures["samples"]
    routes = [part.strip() for part in args.routes.split(",") if part.strip()]
    baseline_url = args.baseline_url or f"{args.fake_base_url}/v1/chat/completions"

    if args.dry_run:
        print(
            json.dumps(
                {
                    "fixtures": str(args.fixtures),
                    "samples": len(samples),
                    "routes": routes,
                    "baseline_url": baseline_url,
                    "litellm_url": args.litellm_url,
                    "passes": args.passes,
                },
                ensure_ascii=False,
                indent=2,
            )
        )
        return 0

    fake_proc: subprocess.Popen[bytes] | None = None
    rows: list[dict[str, Any]] = []
    try:
        if args.start_fake:
            fake_proc = start_fake_server(args.fake_base_url, quiet=args.quiet_fake)
        else:
            wait_for_health(args.fake_base_url, timeout=3)

        if "baseline" in routes:
            rows.extend(
                run_route(
                    "baseline",
                    baseline_url,
                    args.baseline_api_key,
                    samples,
                    args.fake_base_url,
                    args.passes,
                    args.timeout,
                )
            )

        if "litellm" in routes:
            litellm_ok = route_available(args.litellm_url, timeout=3)
            reset_fake(args.fake_base_url, timeout=args.timeout)
            if litellm_ok:
                rows.extend(
                    run_route(
                        "litellm",
                        args.litellm_url,
                        args.litellm_api_key,
                        samples,
                        args.fake_base_url,
                        args.passes,
                        args.timeout,
                    )
                )
            elif args.require_litellm:
                raise BenchmarkError(f"LiteLLM route unavailable: {args.litellm_url}")
            elif not args.skip_litellm_if_unavailable:
                raise BenchmarkError(f"LiteLLM route unavailable: {args.litellm_url}")
            else:
                print(f"LiteLLM route unavailable, skipped: {args.litellm_url}", file=sys.stderr)

        summary = summarize(rows, args.target_hit_rate)
        write_outputs(args.output_dir, rows, summary)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return 0
    finally:
        stop_process(fake_proc)


if __name__ == "__main__":
    try:
        raise SystemExit(main(sys.argv[1:]))
    except BenchmarkError as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(2)
