# toolprobe

**CI for tool calling.** A small Go CLI that probes LLM tool-calling reliability against OpenAI-compatible `chat/completions` endpoints (with tools). Assert correct tool names and JSON arguments. Exit 0 on pass, 1 on fail — drop it into a pipeline.

Built in Berlin by an AI engineer who got tired of “it usually calls the tool.”

## Why

Models drift. Prompts change. Providers disagree on tool schemas. You need a regression harness for *did it call the right tool with the right args?* — not another demo chat UI.

## Install

```bash
go install github.com/chandrasekar-r/toolprobe/cmd/toolprobe@latest
```

Or build from source:

```bash
git clone https://github.com/chandrasekar-r/toolprobe.git
cd toolprobe
go build -o toolprobe ./cmd/toolprobe
```

## Quick start (mock / dry-run)

No API key, no network — validates the harness and default probe:

```bash
toolprobe run --mock --probes probes/default
# or from repo root after build:
./toolprobe run --mock
```

Exit code `0` if all probes pass.

## Live run

```bash
export TOOLPROBE_API_KEY=sk-...
toolprobe run \
  --base-url https://api.openai.com/v1 \
  --model gpt-4o-mini \
  --probes probes/default
```

Flags also accept `--api-key`. JSON report:

```bash
toolprobe run --mock --json
```

## Probe format

YAML under `probes/` (see `probes/default/weather_city_units.yaml`):

```yaml
name: weather_city_units
user: What is the weather in Berlin in celsius?
tools:
  - name: get_weather
    description: Get the current weather for a city.
    parameters:
      type: object
      properties:
        city: { type: string }
        units: { type: string, enum: [celsius, fahrenheit] }
      required: [city, units]
expect:
  tool_name: get_weather
  args:
    city: Berlin
    units: celsius
```

Expected args are matched as a subset (extra keys from the model are OK).

## Layout

```
cmd/toolprobe/          CLI (cobra)
internal/client/        OpenAI-compatible chat + tools (+ mock)
internal/probes/        Probe / ProbeResult, YAML load, runner
internal/score/         Pass rate + latency aggregate
internal/report/        JSON report
probes/default/         Shipping probes
```

## License

MIT
