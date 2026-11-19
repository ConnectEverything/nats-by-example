# NATS by Example

Runnable NATS examples in many languages, published at natsbyexample.com.

- `examples/<category>/<example>/<client>/` — one client implementation. The
  main file's comments plus `meta.yaml` are the published docs; `output.txt`
  and `output.cast` are recordings of a real run and are generated, not edited.
- `docker/<lang>/` — shared Dockerfile and dependency manifest per language,
  merged with the client directory to build each example's image.
- `cmd/nbe/` — the `nbe` CLI: `nbe run <example>`, `nbe verify`, `nbe build`
  (site), `nbe generate recording`. Install with `go install ./cmd/nbe`.

## Checks

- `go test ./cmd/nbe`
- `nbe verify <lang|glob|all>` — builds and runs examples in Docker and flags
  failures and output drift. Run it for every example a change can affect.
- `scripts/check-go-versions.sh` — go.mod `go` directives vs `golang:` images.

## Upgrading dependencies

Follow `.claude/skills/upgrade-examples/SKILL.md`. In short: one language at a
time, baseline with `nbe verify` first, keep code comments and `meta.yaml`
consistent with what the example now does, re-record with
`nbe verify --record`, and never weaken a check to get a pass.
