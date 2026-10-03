# Contributing to toolprobe

Thanks for helping make tool-calling CI less flaky.

## Dev setup

```bash
git clone https://github.com/chandrasekar-r/toolprobe.git
cd toolprobe
go test ./...
go run ./cmd/toolprobe run --mock
```

## Providers

`toolprobe run --provider` is `openai` (default), `anthropic`, `gemini`, or `cloudflare`.

- OpenAI `--mock` replays `probe.mock` in-process and does not build an HTTP request. Anthropic, Gemini, and Cloudflare `--mock` build that provider's native request and parse a local fixture through the provider client. The transport does not dial. A passing mock run is not a live provider result. Do not call live models from tests or CI.
- Native HTTP shapes live in `internal/provider/`. Recorded responses are `internal/provider/testdata/`. Add a fixture when you change a request or response field.
- Do not drop tools to fit a model that cannot call them. Workers AI must return an error when the result is not a tool-calling payload.
- Do not commit API keys. Read them from the env vars in the README.
- All default probes run on all four providers. If a new probe truly cannot be expressed on one provider, skip it in the runner with a documented reason and name it in the README coverage section. Do not rename tools that the provider can express.

Mock check for each provider:

```bash
go run ./cmd/toolprobe run --provider anthropic --mock --baseline toolprobe-baseline.json
go run ./cmd/toolprobe run --provider gemini --mock --baseline toolprobe-baseline.json
go run ./cmd/toolprobe run --provider cloudflare --mock --baseline toolprobe-baseline.json
```

## Adding a probe

1. Drop a YAML file under `probes/default/` (see existing files).
2. Include `mock:` so CI stays deterministic without an API key.
3. Re-write the checked-in baseline if overall coverage grows:

```bash
go run ./cmd/toolprobe run --mock --write-baseline toolprobe-baseline.json
```

4. Update the probe table in `README.md`.
5. `go test ./...` must pass.

## Probe checklist

- Clear `name` / `description`
- `tools` schema matches what you assert in `expect`
- Prefer asserting required args; optional extras are OK (subset match)
- For refusals: `expect.no_tool: true` + mock `mode: text`
- For latency: `expect.max_latency_ms` + mock `latency_ms`

## PRs

- Keep v0.1 lean — no heavy frameworks in the HTML scorecard
- Prefer one focused change per PR
- CI must stay green (tests + mock run + baseline)

## License

By contributing you agree your work is MIT-licensed like the rest of the repo.
