# toolprobe

[![CI](https://github.com/chandrasekar-r/toolprobe/actions/workflows/toolprobe.yml/badge.svg)](https://github.com/chandrasekar-r/toolprobe/actions/workflows/toolprobe.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)

**CI for tool calling.** A small Go CLI that probes LLM tool-calling reliability. It speaks OpenAI-compatible `chat/completions` and, as separate providers, the Anthropic Messages API, Gemini `generateContent`, and Cloudflare Workers AI `/ai/run`. Assert correct tool names and JSON arguments. Exit 0 on pass, 1 on fail — drop it into a pipeline.

Built in Berlin by an AI engineer who got tired of “it usually calls the tool.”

## Install

```bash
go install github.com/chandrasekar-r/toolprobe/cmd/toolprobe@latest
```

From source:

```bash
git clone https://github.com/chandrasekar-r/toolprobe.git
cd toolprobe
go build -o toolprobe ./cmd/toolprobe
```

### Snapshot / release binaries (GoReleaser)

Multi-arch archives (linux / darwin / windows × amd64 / arm64):

```bash
# requires: go install github.com/goreleaser/goreleaser/v2@latest
goreleaser release --snapshot --clean
# → dist/toolprobe_*_{Linux,Darwin,Windows}_{x86_64,arm64}.{tar.gz,zip}
tar -xzf dist/toolprobe_*_Linux_x86_64.tar.gz
./toolprobe run --mock
```

GitHub Releases (when tagged): download the archive for your OS/arch from
https://github.com/chandrasekar-r/toolprobe/releases — checksums in `checksums.txt`.

## Quick start (mock)

`--mock` never opens a network connection. It replays each probe's `mock:` block, for every provider. The checked-in baseline is the OpenAI mock run; the same probes pass for the other providers because expectations do not change.

```bash
toolprobe run --mock --probes probes/default
toolprobe run --mock --repeat 3 --min-pass 1.0 \
  --baseline toolprobe-baseline.json \
  --html scorecard.html
toolprobe report --last            # → scorecard.html from last JSON run

# Same probes, provider selected, still no network:
toolprobe run --provider anthropic --mock --probes probes/default \
  --baseline toolprobe-baseline.json
toolprobe run --provider gemini --mock --probes probes/default \
  --baseline toolprobe-baseline.json
toolprobe run --provider cloudflare --mock --probes probes/default \
  --baseline toolprobe-baseline.json
```

Example output:

```
[PASS] weather_city_units (0.1ms)
[PASS] enum_status_filter (0.0ms)
[PASS] nested_address_geocode (0.0ms)
...
17/17 passed (100%)  avg latency 0.1ms  min-pass 100%
baseline OK (toolprobe-baseline.json)
wrote scorecard.html
```

## Providers

Pick one with `--provider` or `TOOLPROBE_PROVIDER`. Default is `openai`.

| Provider | `--provider` | Live endpoint | Auth env (first match wins) | Account |
|----------|----------------|---------------|------------------------------|---------|
| OpenAI-compatible chat/completions | `openai` | `--base-url` + `/chat/completions` (required) | `--api-key`, else `TOOLPROBE_API_KEY`, else `OPENAI_API_KEY` | — |
| Anthropic Messages tool use | `anthropic` (alias `claude`) | `https://api.anthropic.com/v1/messages` | `--api-key`, else `TOOLPROBE_ANTHROPIC_API_KEY`, else `ANTHROPIC_API_KEY`, else `TOOLPROBE_API_KEY` | — |
| Gemini `generateContent` function calling | `gemini` (alias `google`) | `https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent` | `--api-key`, else `TOOLPROBE_GEMINI_API_KEY`, else `GEMINI_API_KEY`, else `GOOGLE_API_KEY`, else `TOOLPROBE_API_KEY` | — |
| Workers AI traditional function calling | `cloudflare` (alias `workers-ai`) | `https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/{model}` | `--api-key`, else `TOOLPROBE_CLOUDFLARE_API_TOKEN`, else `CLOUDFLARE_API_TOKEN`, else `TOOLPROBE_API_KEY` | `--account-id`, else `TOOLPROBE_CLOUDFLARE_ACCOUNT_ID`, else `CLOUDFLARE_ACCOUNT_ID` |

`--api-key` always overrides the env vars. There is no built-in key. `--base-url` / `TOOLPROBE_BASE_URL` overrides the origin above. Anthropic sends `anthropic-version: 2023-06-01` and `max_tokens: 1024`. Gemini sends the key in the `x-goog-api-key` header.

`--model` / `TOOLPROBE_MODEL` is the model id. OpenAI defaults to `gpt-4o-mini`. Anthropic, Gemini, and Cloudflare do not default a model id on live runs — pass the id you intend to probe. toolprobe has not verified those ids with a live call.

Examples below are ids that appear in each vendor's public docs or catalog (retrieved 2026-10-03), not a claim that this repo ran them:

- Anthropic Messages tool-use docs show ids such as `claude-sonnet-4-5`.
- Gemini `generateContent` function-calling docs show `gemini-3.8-flash`.
- Workers AI catalog marks `@cf/meta/llama-4-scout-17b-16e-instruct` as Function calling. The traditional function-calling guide still shows `@hf/nousresearch/hermes-2-pro-mistral-7b`, which the catalog marks deprecated.

Workers AI tools are sent in the traditional shape (`name`, `description`, `parameters`) on `/ai/run`. If the model returns a plain string, a non-chat result, or an error that it does not support tools, the probe fails with that reason. Tools are not removed from the request to make an unsupported model succeed. A text result that includes `"tool_calls": []` is a real no-tool turn. A text result that omits `tool_calls` entirely still counts as no tool call, and tool-expecting probes fail with a note that the model may have ignored tools.

## Live run

OpenAI-compatible:

```bash
export TOOLPROBE_API_KEY=sk-...
toolprobe run \
  --provider openai \
  --base-url https://api.openai.com/v1 \
  --model gpt-4o-mini \
  --probes probes/default \
  --repeat 3 \
  --min-pass 0.8 \
  --baseline toolprobe-baseline.json \
  --html scorecard.html
```

Anthropic:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
toolprobe run \
  --provider anthropic \
  --model claude-sonnet-4-5 \
  --probes probes/default \
  --min-pass 0.8
```

Gemini (`--model` is required; the id below is the one in Google's public generateContent function-calling example, not a live-verified default):

```bash
export GEMINI_API_KEY=...
toolprobe run \
  --provider gemini \
  --model gemini-3.8-flash \
  --probes probes/default \
  --min-pass 0.8
```

Cloudflare Workers AI (no Workers AI call is made by tests or by `--mock`):

```bash
export CLOUDFLARE_API_TOKEN=...
export CLOUDFLARE_ACCOUNT_ID=...
toolprobe run \
  --provider cloudflare \
  --account-id "$CLOUDFLARE_ACCOUNT_ID" \
  --model @cf/meta/llama-4-scout-17b-16e-instruct \
  --probes probes/default \
  --min-pass 0.8
```

## Probes each provider can run

All 17 default probes run on `openai`, `anthropic`, `gemini`, and `cloudflare`. None are skipped. Tool names are unchanged (`get_weather`, `web_search`, `create_order`, and the rest). Parallel calls (`parallel_two_cities`), nested objects and arrays, enums, and no-tool refusals are expressed in each native format:

- OpenAI: `tools[].function` and `tool_calls[].function.arguments` (JSON string)
- Anthropic: `tools[].input_schema` and `content[]` blocks with `type: tool_use`
- Gemini: `tools[].functionDeclarations` and `parts[].functionCall`
- Workers AI: `tools[]` with `name` / `parameters`, and `result.tool_calls[]`

## Baseline (regression gate)

Write a baseline after a known-good run:

```bash
toolprobe run --mock --write-baseline toolprobe-baseline.json
```

CI compares pass rates (overall + per probe). If a probe that used to pass starts failing, exit 1:

```bash
toolprobe run --mock --baseline toolprobe-baseline.json
```

## HTML scorecard

Every `run` saves `.toolprobe/last-report.json`. Render a static scorecard (no JS/React):

```bash
toolprobe report --last --out scorecard.html
# or during run:
toolprobe run --mock --html scorecard.html
```

## Default probes

| Probe | What it checks |
|-------|----------------|
| `weather_city_units` | Correct tool + city/units args |
| `weather_fahrenheit` | Multi-arg units variant |
| `optional_units_omitted` | Required vs optional (units omitted) |
| `search_query_limit` | Multi-arg schema (query, limit, language) |
| `calculator_expression` | Simple expression tool |
| `enum_status_filter` | Enum arg (`open\|closed\|all`) |
| `enum_sort_direction` | Enum + required multi-arg sort |
| `nested_address_geocode` | Nested object (`address.*`) |
| `nested_order_items` | Nested array of objects (line items) |
| `select_weather_not_search` | Multi-tool selection |
| `refuse_unknown_capability` | No tool when capability missing |
| `refuse_invented_tool` | Refuse invented tool names |
| `parallel_two_cities` | Parallel multi-tool calls |
| `calendar_create_event` | Multi-arg datetime create |
| `email_send_fields` | Multi-arg to/subject/body |
| `translate_text_lang` | Multi-arg translate |
| `latency_budget_fast` | `max_latency_ms` budget |

Empty / malformed args and wrong-tool paths are covered in unit tests via mock modes `empty_args`, `malformed_args`, `wrong_tool`.

## CI

GitHub Actions: `.github/workflows/toolprobe.yml`

```yaml
- run: go test ./...
- run: go run ./cmd/toolprobe run --mock --baseline toolprobe-baseline.json --html scorecard.html
- run: go run ./cmd/toolprobe run --provider anthropic --mock --baseline toolprobe-baseline.json
- run: go run ./cmd/toolprobe run --provider gemini --mock --baseline toolprobe-baseline.json
- run: go run ./cmd/toolprobe run --provider cloudflare --mock --baseline toolprobe-baseline.json
- run: go run ./cmd/toolprobe report --last
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Release

`.goreleaser.yml` builds linux/darwin/windows (`amd64`/`arm64`, CGO off). Snapshot:

```bash
goreleaser release --snapshot --clean
```

## Layout

```
cmd/toolprobe/           CLI — run / report (--provider)
internal/client/         OpenAI-compatible chat + tools
internal/provider/       anthropic, gemini, cloudflare, openai adapter
internal/probes/         Probe / mock / runner
internal/baseline/       Regression baseline
internal/score/          Pass rate + latency
internal/report/         JSON + static HTML scorecard
probes/default/          Shipping probes (17)
deploy/toolprobe.rclabs.in/  Static site source for Site Ops (not deployed from CI)
.goreleaser.yml          Multi-arch binaries
toolprobe-baseline.json  Checked-in mock baseline
CONTRIBUTING.md          How to add probes
```

## Static page for toolprobe.rclabs.in

`deploy/toolprobe.rclabs.in/` is a static Worker (assets only: no AI binding, no KV, no R2, no container, no API proxy). It does not hold keys and does not call Workers AI. See [deploy/toolprobe.rclabs.in/README.md](deploy/toolprobe.rclabs.in/README.md) for the exact `wrangler deploy` steps. This repo does not deploy it.

## License

MIT
