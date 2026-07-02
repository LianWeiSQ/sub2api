# Sub2API Cache Benchmark

This benchmark is offline-safe. It uses a deterministic OpenAI-compatible fake
LLM so cache behavior can be tested without real provider keys.

## Samples

`fixtures.json` contains exactly 10 fixed chat completion samples:

- 2 short system-prompt tasks
- 2 short tool-schema tasks
- 2 complex system-prompt tasks
- 3 complex tool-schema tasks
- 1 streaming control task

The streaming sample is marked `cacheable: false` because many gateways only
store streaming responses after all chunks complete, or bypass streaming cache.

## Fake LLM

Start only the fake upstream:

```bash
python3 tools/cache_benchmark/fake_openai_llm.py --port 18081
```

Useful endpoints:

```text
GET  http://127.0.0.1:18081/health
GET  http://127.0.0.1:18081/metrics
POST http://127.0.0.1:18081/reset
POST http://127.0.0.1:18081/v1/chat/completions
```

## Offline Smoke Run

This starts the fake LLM, runs the baseline route directly against it, and skips
LiteLLM if it is not running:

```bash
python3 tools/cache_benchmark/run_cache_benchmark.py \
  --start-fake \
  --routes baseline,litellm
```

Outputs are written to:

```text
tools/cache_benchmark/results/cache_benchmark_results.json
tools/cache_benchmark/results/cache_benchmark_results.csv
tools/cache_benchmark/results/cache_benchmark_summary.json
```

## LiteLLM Cache Proxy Run

Install or run LiteLLM separately, then point it at the fake LLM with:

```bash
FAKE_OPENAI_BASE_URL=http://127.0.0.1:18081/v1 \
FAKE_OPENAI_API_KEY=fake-key \
litellm --config deploy/litellm-cache-benchmark.config.yaml --port 4000
```

In another terminal:

```bash
python3 tools/cache_benchmark/run_cache_benchmark.py \
  --start-fake \
  --require-litellm \
  --litellm-url http://127.0.0.1:4000/v1/chat/completions
```

The template master key is `sk-bench-key`, which is also the runner default for
`--litellm-api-key`.

With the default `--passes 10`, an exact cache should miss once and hit nine
times for each cacheable sample, which produces a 90% target hit rate.

## Sub2API Baseline Run

After configuring a Sub2API OpenAI-compatible account whose upstream base URL is
the fake LLM, pass the Sub2API endpoint as the baseline URL:

```bash
SUB2API_BENCHMARK_URL=http://127.0.0.1:3000/openai/v1/chat/completions \
SUB2API_BENCHMARK_API_KEY=your-local-sub2api-key \
python3 tools/cache_benchmark/run_cache_benchmark.py \
  --start-fake \
  --routes baseline
```

The runner infers cache hits from fake upstream call deltas. If a request returns
without increasing `/metrics.upstream_call_count`, it is counted as a gateway
cache hit.
