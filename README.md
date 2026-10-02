# toolprobe

[![CI](https://github.com/chandrasekar-r/toolprobe/actions/workflows/toolprobe.yml/badge.svg)](https://github.com/chandrasekar-r/toolprobe/actions/workflows/toolprobe.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)

**CI for tool calling.** A small Go CLI that probes LLM tool-calling reliability against OpenAI-compatible `chat/completions` endpoints (with tools). Assert correct tool names and JSON arguments. Exit 0 on pass, 1 on fail — drop it into a pipeline.

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

```bash
toolprobe run --mock --probes probes/default
toolprobe run --mock --repeat 3 --min-pass 1.0 \
  --baseline toolprobe-baseline.json \
  --html scorecard.html
toolprobe report --last            # → scorecard.html from last JSON run
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

## Live run

```bash
export TOOLPROBE_API_KEY=sk-...
toolprobe run \
  --base-url https://api.openai.com/v1 \
  --model gpt-4o-mini \
  --probes probes/default \
  --repeat 3 \
  --min-pass 0.8 \
  --baseline toolprobe-baseline.json \
  --html scorecard.html
```

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
cmd/toolprobe/           CLI — run / report
internal/client/         OpenAI-compatible chat + tools
internal/probes/         Probe / mock / runner
internal/baseline/       Regression baseline
internal/score/          Pass rate + latency
internal/report/         JSON + static HTML scorecard
probes/default/          Shipping probes (17)
.goreleaser.yml          Multi-arch binaries
toolprobe-baseline.json  Checked-in mock baseline
CONTRIBUTING.md          How to add probes
```

## License

MIT
