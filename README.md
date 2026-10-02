# toolprobe

**CI for tool calling.** A small Go CLI that probes LLM tool-calling reliability against OpenAI-compatible `chat/completions` endpoints (with tools). Assert correct tool names and JSON arguments. Exit 0 on pass, 1 on fail — drop it into a pipeline.

Built in Berlin by an AI engineer who got tired of “it usually calls the tool.”

## Install

```bash
go install github.com/chandrasekar-r/toolprobe/cmd/toolprobe@latest
```

From source / release binaries (via GoReleaser):

```bash
git clone https://github.com/chandrasekar-r/toolprobe.git
cd toolprobe
go build -o toolprobe ./cmd/toolprobe

# multi-arch archives (requires goreleaser):
# goreleaser release --snapshot --clean
```

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
[PASS] select_weather_not_search (0.0ms)
[PASS] refuse_unknown_capability (0.0ms)
...
12/12 passed (100%)  avg latency 0.1ms  min-pass 100%
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
| `search_query_limit` | Multi-arg schema (query, limit, language) |
| `calculator_expression` | Simple expression tool |
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
probes/default/          Shipping probes
.goreleaser.yml          Multi-arch binaries
toolprobe-baseline.json  Checked-in mock baseline
```

## License

MIT
