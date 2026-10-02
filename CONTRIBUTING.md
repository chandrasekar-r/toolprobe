# Contributing to toolprobe

Thanks for helping make tool-calling CI less flaky.

## Dev setup

```bash
git clone https://github.com/chandrasekar-r/toolprobe.git
cd toolprobe
go test ./...
go run ./cmd/toolprobe run --mock
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
