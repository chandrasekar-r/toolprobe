# toolprobe

**CI for tool calling.** A small Go CLI that probes LLM tool-calling reliability against OpenAI-compatible `chat/completions` endpoints (with tools). Assert correct tool names and JSON arguments. Exit 0 on pass, 1 on fail — drop it into a pipeline.

Built in Berlin by an AI engineer who got tired of “it usually calls the tool.”

## Why

Models drift. Prompts change. Providers disagree on tool schemas. You need a regression harness for *did it call the right tool with the right args?* — not another demo chat UI.

## Install

```bash
go install github.com/chandrasekar-r/toolprobe/cmd/toolprobe@latest
```

Or from source:

```bash
git clone https://github.com/chandrasekar-r/toolprobe.git
cd toolprobe
go build -o toolprobe ./cmd/toolprobe
```

## Quick start (mock / dry-run)

No API key, no network — each probe’s `mock:` block drives the response:

```bash
toolprobe run --mock --probes probes/default
./toolprobe run --mock --repeat 3 --min-pass 1.0
```

Exit code `0` if pass rate ≥ `--min-pass` (default `1.0`).

## Live run

```bash
export TOOLPROBE_API_KEY=sk-...
toolprobe run \
  --base-url https://api.openai.com/v1 \
  --model gpt-4o-mini \
  --probes probes/default \
  --repeat 3 \
  --min-pass 0.8
```

JSON report: `toolprobe run --mock --json`.

## Default probes (`probes/default/`)

| Probe | What it checks |
|-------|----------------|
| `weather_city_units` | Correct tool + city/units args |
| `weather_fahrenheit` | Multi-arg units variant |
| `search_query_limit` | Multi-arg schema (query, limit, language) |
| `calculator_expression` | Simple expression tool |
| `select_weather_not_search` | Multi-tool selection (pick weather, not search) |
| `refuse_unknown_capability` | No tool when capability missing |
| `refuse_invented_tool` | Refuse invented / unknown tool names |
| `parallel_two_cities` | Parallel multi-tool (two weather calls) |
| `calendar_create_event` | Multi-arg datetime event create |
| `email_send_fields` | Multi-arg to/subject/body |
| `translate_text_lang` | Multi-arg translate |
| `latency_budget_fast` | `max_latency_ms` budget (mockable) |

Empty / malformed args and wrong-tool paths are covered in unit tests (`internal/probes`) via mock modes `empty_args`, `malformed_args`, and `wrong_tool`.

## Probe format

```yaml
name: weather_city_units
user: What is the weather in Berlin in celsius?
tools:
  - name: get_weather
    parameters:
      type: object
      properties:
        city: { type: string }
        units: { type: string }
      required: [city, units]
expect:
  tool_name: get_weather
  args: { city: Berlin, units: celsius }
  # optional: no_tool: true | calls: [...] | max_latency_ms: 200
mock:
  mode: tool_calls   # tool_calls | text | empty_args | malformed_args | wrong_tool
  tool_name: get_weather
  args: { city: Berlin, units: celsius }
```

Expected args are matched as a subset (extra keys OK). Multi-call expects are order-insensitive.

## CI

GitHub Actions workflow: `.github/workflows/toolprobe.yml`

```yaml
- run: go test ./...
- run: go run ./cmd/toolprobe run --mock --probes probes/default --min-pass 1.0
```

## Layout

```
cmd/toolprobe/          CLI (cobra) — --mock --repeat --min-pass --json
internal/client/        OpenAI-compatible chat + tools
internal/probes/        Probe / mock / runner / YAML load
internal/score/         Pass rate + latency aggregate
internal/report/        JSON report
probes/default/         Shipping probes
.github/workflows/      CI
```

## License

MIT
