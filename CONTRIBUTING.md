# Contributing to ProtoMock

Thanks for helping! This guide explains how the code fits together, how to run and test it, and what to touch for common changes.

- [Setup](#setup)
- [Architecture](#architecture)
- [Tests](#tests)
- [Working on the UI](#working-on-the-ui)
- [Common changes](#common-changes)
- [CI and releases](#ci-and-releases)

---

## Setup

You need **Go**, at the version in `go.mod`. These are optional:
- [`golangci-lint`](https://golangci-lint.run/) v2 for `make lint`;
- [`grpcurl`](https://github.com/fullstorydev/grpcurl) to poke at gRPC mocks;
- Docker.

```bash
make run    # serves ./mocks on :8085 (HTTP + UI) and :8086 (gRPC)
make help   # (or read the Makefile) for every target
```

## Architecture

```text
cmd/server/            main: parse config, run the app
internal/
├── config/            flags / env vars / defaults
├── app/               wiring: load mocks, run servers, hot reload (watcher + Reload)
├── loader/            mocks folder → routes; all stub rules (BuildRoute); proto compiling; JSON templates
├── models/            Stub (the file format) and Route (a loaded, ready-to-serve stub)
├── match/             shared matching rules: partial JSON body, regex URLs
├── httpserver/        HTTP router (exact → regex → 404), matching, responses
├── grpcserver/        catch-all gRPC handler, matching, status errors, reflection
├── store/             safe file writes for the UI (name validation, atomic writes)
└── web/               UI (embedded files in web/ui/) and its JSON API
test/
├── integration/       real loader + servers on random ports (+ __snapshots__/)
└── benchmark/         router benchmarks
```

### Dependency direction

```text
cmd/server → app → httpserver, grpcserver, web, loader, config
web        → loader, store
loader     → models, match
httpserver, grpcserver → models, match
```

The servers know nothing about files, and the loader knows nothing about serving. `app` is the only package that connects them.

### Loading and hot reload

1. `loader.LoadAll(dir)` walks `mocks/http` and `mocks/grpc` once. For each mock folder it:
   - compiles the `.proto` files (`CompileProtos`),
   - turns every `stubs/*.json` into a `models.Route` with `BuildRoute`.

   Invalid files become `Set.Problems` entries; they never stop the load.
2. `app.App` holds the current `loader.Set`, and calls `Update(routes)` on both servers.
   - Each server keeps its routes behind an `atomic.Pointer`, so a reload is a lock-free swap.
   - In-flight requests finish on the old routes.
3. Reloads happen:
   - every `-reload-interval`, when a fingerprint of the folder (paths, sizes, mtimes) changes;
   - after every UI write (`App.Locked`);
   - on `POST /protomock-ui/api/reload`.

### How a request is served

**HTTP** ([internal/httpserver](internal/httpserver)):
1. `/protomock-ui[/...]` goes to the UI. Everything else goes to the mock handler.
2. **Exact URL** stubs for the path are tried first, in priority order. Then the **regex** stubs.
3. A stub matches on method, headers and partial body (`match.PartialJSON`). The request body is read once.
4. `writeResponse`: wait for `delay_ms`, then set headers, write the status, write the JSON or protobuf body.

**gRPC** ([internal/grpcserver](internal/grpcserver)):
1. There are no generated services. `grpc.UnknownServiceHandler` receives every call.
2. The stubs for the full method name are looked up. The request is received **once**, decoded with the method's input type (`dynamicpb`).
3. Metadata and body are matched. The body is checked as JSON with both proto and camelCase field names.
4. The reply is the stub's prebuilt `dynamicpb` message, with metadata and delay. No match returns a status error (`NotFound` / `Unimplemented`).
5. Reflection (`reflection.go`) describes the mocked services from the loaded descriptors.

### Conventions

- Stub validation lives in **one place**, `loader.BuildRoute`. The loader, the UI's validate and save, and the docs test all use it.
- Return errors; only `main` exits. Log with the existing emoji prefixes (`[HTTP]`, `[gRPC]`, `[UI]`, `⚠️`, `❌`).
- Keep packages small, with a doc comment that says what the package is for.
- Match the surrounding code style. `gofmt` and `golangci-lint` are enforced in CI.

## Tests

| Kind | Where | Run | Notes |
|---|---|---|---|
| **Unit** | next to the code, `internal/**/*_test.go` | `make test` | Fast, no network. The default for `go test ./...`. |
| **Integration** | `test/integration/` (build tag `integration`) | `make test-integration` | Real loader, servers and app on random local ports, against mocks written to `t.TempDir()`. |
| **Benchmark** | `test/benchmark/` (build tag `bench`) | `make bench` | |
| **Everything CI runs** | | `make ci` | Tidy check, vet, lint, unit and integration with `-race`. |

Build tags keep the slow test kinds out of `go test ./...`.

**Unit test style:**
- Table-driven tests, using `t.Run(name, ...)`.
- Test private functions directly; the files are in the same package.
- Use `httptest` for HTTP handlers.

**Integration test helpers** live in `test/integration/`:
- `writeFiles` writes a mocks tree,
- `startApp` runs the whole app on random ports,
- `startGreeter` runs a gRPC mock,
- `compileProto` builds messages for a client,
- `eventually` polls for async changes like hot reload.

**Snapshot tests** (`go-snaps`) cover large API responses, in `test/integration/__snapshots__/`.
- After an **intended** change in output, refresh and review the snapshots:
  ```bash
  make snapshots   # UPDATE_SNAPS=true go test -tags integration ./test/integration/...
  git diff test/integration/__snapshots__
  ```
- CI fails if a snapshot is missing or different.

**Docs test:** `TestDocs_StubExamplesAreValid` validates every JSON example in [docs/stubs.md](docs/stubs.md). If you add an example, it has to be a valid stub.

**Fixing a bug:**
1. Write a test that fails because of the bug.
2. Fix the code.
3. Watch the test pass.

## Working on the UI

The UI is plain HTML, CSS and JavaScript in [internal/web/ui](internal/web/ui). There's no build step and no dependencies. The files are embedded in the binary with `go:embed`.

```bash
make dev-ui   # go run ./cmd/server -web-dir internal/web/ui
```

`-web-dir` serves the files from disk, so edits show up on a browser refresh.

Guidelines:
- **Never use `innerHTML` with data.** Build elements with `h(tag, attrs, ...children)`, which inserts text safely.
- **The stub JSON text is the source of truth.** The form edits `ed.obj` and re-renders the text, and the JSON tab parses the text back. Unknown keys must survive a round trip.
- **Ask the API, don't re-implement rules.** For example, `/validate` for stub rules, and `/template` for message templates.
- **Check both themes and a narrow window:** light and dark (`prefers-color-scheme`), and about 800 px wide.

The JSON API is in [internal/web/api.go](internal/web/api.go), documented in [docs/ui.md](docs/ui.md#json-api), and tested in `test/integration/editor_api_test.go`.

## Common changes

### Add a stub field

1. `models.Stub` / `models.ResponseStub`: add the JSON field.
2. `models.Route`: add what the servers need at serve time.
3. `loader.BuildRoute`: validate the field and copy it into the route. Add cases to `internal/loader/route_test.go`.
4. `httpserver` / `grpcserver`: use it, with a unit or integration test.
5. UI: add a form input in `formEditor` in `app.js`.
6. Docs: add it to the field table and an example in [docs/stubs.md](docs/stubs.md). The docs test checks the example.

### Add a config option

Add it in `internal/config/config.go` (a flag plus a `PROTOMOCK_*` env var), test the precedence in `config_test.go`, use it in `internal/app`, and document it in the README's Configuration table.

### Add a UI API endpoint

Register it in `api.register`. Route file writes through `a.backend.Locked(...)` and `store`, return `writeMocks` after writes, and add an integration test.

## CI and releases

- **CI** ([.github/workflows/ci.yml](.github/workflows/ci.yml)) runs on every push to `main` and every pull request:
  - lint (tidy check, vet, `golangci-lint`),
  - unit tests with `-race` and coverage,
  - integration tests with `-race`,
  - a Docker build.
- **Releases:** publishing a GitHub release (e.g. tag `v2.3.0`) runs [.github/workflows/release.yml](.github/workflows/release.yml), which builds and pushes `avijeet7/protomock:<tag>` and `:latest` for amd64 and arm64.
- **Versioning:** [semantic versioning](https://semver.org/).
  - New options and features bump the minor version.
  - Fixes bump the patch version.
  - Removing or changing documented behaviour bumps the major version.
