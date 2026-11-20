---
name: upgrade-examples
description: Safely upgrade NATS by Example dependencies (client libraries, nats-server/CLI tools, Docker base images) for one language at a time, verify every affected example still builds, runs and does what its prose claims, and re-record outputs. Use when asked to update/upgrade/bump dependencies or versions in this repo, when fixing a Dependabot PR or an `example-health` issue, or when examples fail `nbe verify`.
---

# Upgrading the examples

The examples are documentation: `meta.yaml` and the comments in each client's
main file are rendered on natsbyexample.com next to the recorded
`output.txt`/`output.cast`. An upgrade is done only when every affected
example builds, runs, and still demonstrates what its prose says.

## How the repo is wired

- `examples/<category>/<example>/<client>/` holds a client's main file and its
  recording. `nbe` merges `docker/<client>/` (Dockerfile + dependency manifest)
  with the client directory to build the image, then runs it with
  `docker/docker-compose.yaml` (a `nats` server + the app) unless the
  example/client has its own compose file or Dockerfile.
- So most dependency versions are shared per language in `docker/<lang>/`,
  and a single bump there affects every example of that language.

Where versions live:

| What | Where |
|---|---|
| nats-server used by most examples | `docker/docker-compose.yaml`, `docker/docker-compose.cluster.yaml` (`nats:` image) |
| nats-server / natscli / nsc for CLI examples | `go install ...@vX` lines in `docker/cli/Dockerfile` and in `examples/**/Dockerfile` |
| Go | `docker/go/go.mod` (shared; `docker/go/deps.go` is generated), per-example `go.mod`s, `FROM golang:` lines |
| Rust | `docker/rust/Cargo.toml`, `docker/rust/Dockerfile` |
| Python | `docker/python/requirements.txt`, `docker/python/Dockerfile` |
| Node | `docker/node/package.json`, `docker/node/Dockerfile` |
| Deno | URL/JSR imports in each `examples/**/deno/main.js`, `docker/deno/Dockerfile` |
| Java | `docker/java/build.gradle`, `docker/java/Dockerfile`, `examples/**/java/Dockerfile` |
| C# (`csharp`) | each `examples/**/csharp/*.csproj`, `docker/csharp/Dockerfile` |
| .NET v1 client (`dotnet`) | `docker/dotnet/example.csproj`, `docker/dotnet/Dockerfile` |
| Elixir | `Mix.install` in each `examples/**/elixir/main.exs`, `docker/elixir/Dockerfile` |
| Crystal | `docker/crystal/shard.yml` + `shard.lock`, `docker/crystal/Dockerfile` |
| C | `docker/c/install-nats-c.sh`, `docker/c/Dockerfile` |

## Tools

- `nbe verify [all | <lang> | <glob>]...` builds and runs examples and
  reports each one as **fail** (build/run error, timeout), **warn** (ran, but
  the output is empty, shows new error markers, or drifted from the committed
  `output.txt`) or **pass**. Useful flags: `--out DIR` (new outputs and build
  logs), `--json`, `--report`, `--concurrency`, `--record`.
- `nbe report a.json b.json` merges JSON results into one Markdown report.
- `scripts/upgrade-deps.sh` upgrades all Go modules, regenerates
  `docker/go/deps.go`, builds them, runs govulncheck, and runs
  `scripts/check-go-versions.sh` (a go.mod's `go` directive must not exceed
  its Dockerfile's `golang:` image; the images use `GOTOOLCHAIN=local`).
- Install nbe with `go install ./cmd/nbe` after changing `cmd/nbe`.
- Behind a TLS-intercepting proxy (e.g. a sandbox), set
  `NBE_CA_BUNDLE=/path/to/ca-bundle.pem` so image builds trust it. If Docker
  Hub rate-limits pulls, configure a registry mirror such as
  `mirror.gcr.io` in `/etc/docker/daemon.json`.

## Procedure

Work one language (or the server/CLI tools) at a time, on a branch, with one
commit per language.

1. **Baseline.** Run `nbe verify <lang> --out /tmp/before --json /tmp/before.json`
   before changing anything, so you know which failures already existed.
   Fix pre-existing breakage in a separate commit first, or call it out.
2. **Choose versions.** Look up the latest releases from the registry, not
   from memory, and **bias towards the newest**: readers copy these
   examples, and recent versions carry security fixes. That includes major
   versions when the migration is mechanical (renamed packages or methods,
   e.g. Jackson 2 → 3, `jwt-java` → `jwt-utils-jdkNN`), current LTS
   runtimes (Java, .NET, Node) and current stable base images (Debian,
   Ubuntu, Alpine). Don't hold one library back to suit another: find the
   newer release of the dependent instead (e.g. `jwt-java` 2.2.0 needs
   `jnats-json` 2.x, but its successor `jwt-utils-jdk25` works with 3.x).
   When a migration changes what an example teaches (for example nats.js
   v2 `nats` → v3 `@nats-io/*`, which drops the legacy push-consumer API),
   propose it as its own change rather than mixing it into a routine bump,
   and note it in the PR. The .NET `dotnet` examples intentionally show the
   v1 `NATS.Client`; the `csharp` ones show `NATS.Net`. Pre-1.0 crates such
   as `async-nats` treat minor bumps as breaking.
3. **Bump** the manifest, lock file and `FROM` image tags together. Keep base
   images on supported releases (e.g. Go: the two latest minors; .NET: an LTS
   that is in support; Node: an active LTS). Never pin to a tag that doesn't
   exist; check with `docker manifest inspect <image:tag>`.
4. **Verify.** `nbe verify <lang> --out /tmp/after --json /tmp/after.json`.
   - **fail:** read `/tmp/after/<example>/build.log` or `output.txt`. Fix the
     example code with the smallest change that uses the new API idiomatically.
     The comments around changed code are the published docs; update them too.
   - **warn:** diff `/tmp/after/<example>/output.txt` against the committed
     `output.txt`. Re-read the example's `meta.yaml` and comments and decide
     whether the claims still hold (ordering guarantees, delivery counts,
     error messages shown, field names printed). If behavior changed, fix the
     code or update the prose so the two agree. If only cosmetic details
     changed (log formats, version banners, timing), that's fine.
   - **pass:** still skim the output diff when the bump was large.
5. **Re-record** the examples that changed (needs `asciinema` and `nbe` on
   PATH): `nbe verify --record <targets>`. Don't hand-edit `output.txt` or
   `output.cast`.
6. **Check the site still builds:** `go test ./cmd/nbe && nbe build`.
7. **Commit** with a message listing old → new versions, the examples whose
   code or prose changed and why, and anything still failing.

## Pitfalls seen in practice

- **Transitive compatibility.** Before bumping a library that other
  dependencies are built against, check what they expect (e.g. `jwt-java`
  2.2.0 is built against `jnats-json` 2.0.1; forcing 3.x compiles but the
  auth callout handler fails at runtime). Read the dependents' POM/manifest,
  then upgrade the dependent too (here to `jwt-utils-jdkNN`) rather than
  pinning the library back.
- **Compiles but crashes.** C passes mismatched pointer types with only a
  warning; `docker/c/CMakeLists.txt` turns those into errors. In other
  languages, a run that exits 0 with empty output is also a failure (the
  Rust Dockerfile once re-linked a placeholder `main()`); `nbe verify`
  warns on empty output for this reason.
- **CLI flag and output changes.** natscli/nsc releases rename commands
  (`nats bench` → `nats bench js pub ...`), drop flags (`context save --nsc`),
  make commands interactive (`stream view`, which fails without a TTY), and
  move output between stdout and stderr (`nsc generate nkey`). Check
  `--help` of the new version for every command a script uses.
- **Keep semantics, not just syntax.** When replacing a command, check it
  still behaves the same over the whole script, e.g. contexts pointing at
  the creds files nsc rewrites on `nsc edit user`, rather than one-off
  copies that miss later permission changes.
- **Multiple servers on one host** each need their own JetStream
  `store_dir`, or they corrupt each other's RAFT state.
- **Startup races.** A healthy `/healthz` on one server doesn't mean every
  server has joined JetStream. Before creating streams that need placement
  across clusters, wait with `nats server check meta --expect N
  --seen-critical 5s --lag-critical 1000`, and give health-check retries
  enough budget (`curl --retry 30`) for slow CI runners.
- **Libraries that need configuration.** Some upgrades add required setup
  (e.g. `jwt-utils` needs the `nkey.provider.class` property). Make it
  explicit in the example code with a comment, so readers see it too.
- **Run time.** Examples must finish within `nbe`'s one-minute limit on a
  small CI runner; trim message counts in benchmarks rather than raising it.

## Don'ts

- Don't paper over failures: no deleting examples, catching and ignoring
  errors, raising timeouts, or lowering `--min-similarity` to get a pass.
- Don't run `go get -u` / `go mod tidy` in `docker/go` by hand without
  `scripts/sync-go-deps.sh`, or the shared go.mod loses the examples' deps.
- Don't raise a `go` directive above the Dockerfile's `golang:` image.
- Don't mix a server upgrade with client upgrades in one commit: when output
  changes you need to know which side caused it.
