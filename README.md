# 🧪 ProtoMock: Dynamic HTTP & gRPC Mock Server

**ProtoMock** mocks HTTP and gRPC APIs from `.proto` files and JSON stubs. Point it at a folder of mocks, or build them in the web UI, and it serves them immediately: no code generation, no restarts. Use it for integration tests, sandboxes, and frontend or backend development against services that aren't available.

---

## 📦 Features

- ✅ **HTTP and gRPC** from one server (`:8085` HTTP + UI, `:8086` gRPC)
- 💻 **Web UI to create, edit, validate and try mocks**: forms or raw JSON, message templates, live validation
- 🔁 **Hot reload**: changes in the UI or on disk are live within seconds, no restart
- 🧠 **Protobuf without codegen**: `.proto` files are compiled at runtime; JSON stub bodies are converted to protobuf
- 🔍 **WireMock-style matching**: method, exact or regex URL, headers or metadata, partial JSON body
- 🎚 **Priority** for overlapping stubs; **response headers/metadata** and **delays**
- 🛰 **gRPC reflection**: `grpcurl list` / `describe` / call without local `.proto` files
- 🤖 JSON or **protobuf** HTTP responses; plain-JSON HTTP mocks need no `.proto`
- 🐳 Small Docker image (~42 MB, amd64 + arm64), configurable through flags or environment variables

---

## 🚀 Quick Start

### With Docker

```bash
docker run -p 8085:8085 -p 8086:8086 -v $(pwd)/mocks:/app/mocks avijeet7/protomock:latest
```

Open **http://localhost:8085/protomock-ui** and click **+ New mock**.

> The UI saves stubs into the mounted folder. Mount it read-only (`:/app/mocks:ro`) to make the UI read-only.

### From source

```bash
make run          # or: go run ./cmd/server
```

This serves the sample mocks in [`mocks/`](mocks). The startup log prints the URLs to open. Try them:

```bash
curl -i http://localhost:8085/hello-json
curl -X POST http://localhost:8085/hello/http -H "X-Test-Header: mocked3"
grpcurl -plaintext -H "X-Test-Header: mocked1" localhost:8086 test.FakeService/Hello
```

### Next to your services (docker-compose)

```yaml
services:
  protomock:
    image: avijeet7/protomock:v2.3.0
    ports:
      - "8085:8085"   # host:container. Change the host side if 8085 is taken, e.g. "8888:8085"
      - "8086:8086"
    environment:
      PROTOMOCK_PUBLIC_URL: http://localhost:8085   # match the host port: printed as the UI URL
      PROTOMOCK_STRICT: "true"                      # fail fast if a stub or .proto is broken
    volumes:
      - ./integrationtests/protomock/mocks:/app/mocks

  my-service:
    environment:
      FEATURE_FLAGS_URL: http://protomock:8085      # other containers use the container port
    depends_on:
      protomock:
        condition: service_healthy                  # the image has a built-in health check
```

- **Ports:** other containers always reach ProtoMock on the **container** ports (`protomock:8085`, `protomock:8086`). Only your own browser or test runner uses the host ports.
- **Health:** the image has a built-in `HEALTHCHECK` (`protomock healthcheck`), so you don't need a custom one. Existing `curl` health checks keep working too.
- **No `platform: linux/amd64` needed:** the image is multi-arch (amd64 + arm64), so it runs natively on Apple Silicon.

---

## 🧩 How it Works

A **mock** is a folder. A **stub** is one JSON file in its `stubs/` folder: *"when a request looks like this, reply with that"*.

```text
mocks/
├── http/
│   └── users/                  ← an HTTP mock
│       ├── users.proto         ← optional: only for protobuf responses
│       └── stubs/
│           ├── list.json
│           └── get_by_id.json
└── grpc/
    └── shop/                   ← a gRPC mock
        ├── shop.proto          ← services + messages
        └── stubs/
            └── get_item.json
```

An HTTP stub:

```json
{
  "request": {
    "method": "GET",
    "url": "/users/[0-9]+",
    "headers": { "X-Tenant": "acme" }
  },
  "response": {
    "status": 200,
    "headers": { "X-Request-Id": "mock-1" },
    "body": { "id": 1, "name": "Bob" }
  }
}
```

A gRPC stub (the response type comes from the method in `shop.proto`):

```json
{
  "request": {
    "url": "/shop.Shop/GetItem",
    "body": { "id": "42" }
  },
  "response": {
    "body": { "id": "42", "name": "Lamp" }
  }
}
```

**Matching, in short:**
- A stub matches when its method, URL, headers and body match.
  - The URL is exact, or an anchored regex.
  - Body matching is partial: the request must *contain* the stub's fields.
- Among matching stubs, the highest `priority` wins, then file-name order.
- No match returns HTTP `404`, or gRPC `NotFound` / `Unimplemented`, **saying why**: the closest stub and the rule it failed, e.g. `header X-Tenant: expected "acme", got "other"`. The same line goes to the server log.

📖 **Full reference: [docs/stubs.md](docs/stubs.md).** It covers every field, every matching rule, and examples for each case.

---

## ✨ Web UI

Open **http://localhost:8085/protomock-ui**.

| | |
|---|---|
| **Browse** | Every HTTP and gRPC mock and stub, with search and problem markers |
| **Create** | New HTTP or gRPC mocks (with a sample `.proto`), new stubs, or `+ Stub` straight from a gRPC method |
| **Edit** | A form for every stub field, or raw JSON, kept in sync; **Insert template** fills a body with every field of a protobuf message |
| **Validate** | Live, using the same rules as the server, before you save |
| **Try it** | Send a test request and see the response; protobuf HTTP responses are decoded to JSON |
| **Protos** | View and edit `.proto` files; edits must compile before they're saved |

📖 **UI guide and its JSON API (for scripts and CI): [docs/ui.md](docs/ui.md)**

---

## ⚙️ Configuration

Every setting is a flag or an environment variable. **Precedence: flag > environment variable > default.**

| Flag | Environment variable | Default | Description |
|---|---|---|---|
| `-http-addr` | `PROTOMOCK_HTTP_ADDR` | `:8085` | HTTP mocks and UI listen address |
| `-grpc-addr` | `PROTOMOCK_GRPC_ADDR` | `:8086` | gRPC mocks listen address |
| `-mocks-dir` | `PROTOMOCK_MOCKS_DIR` | `mocks` | Folder containing `http/` and `grpc/` mocks |
| `-reload-interval` | `PROTOMOCK_RELOAD_INTERVAL` | `2s` | How often to check the mocks folder for changes (`0` disables) |
| `-strict` | `PROTOMOCK_STRICT` | `false` | Fail startup if any stub or `.proto` doesn't load; the health check is unhealthy while any doesn't. Recommended for CI. |
| `-public-url` | `PROTOMOCK_PUBLIC_URL` | *(listen address)* | Base URL printed for the UI at startup, e.g. `http://localhost:8888` when Docker maps a different host port |
| `-web-dir` | `PROTOMOCK_WEB_DIR` | *(built in)* | Serve UI files from a folder instead (for UI development) |

```bash
go run ./cmd/server -http-addr :9085 -grpc-addr :9086
docker run -p 9085:9085 -e PROTOMOCK_HTTP_ADDR=:9085 avijeet7/protomock:latest
```

- **Port in use:** ProtoMock exits with an error instead of running half-started.
- **Paths:** the UI and its API live under `/protomock-ui/`; every other path is free for your mocks.
- **Startup URLs:** the startup log prints the URLs for the ports ProtoMock actually listens on. Inside a container it can't see your compose port mapping, so set `PROTOMOCK_PUBLIC_URL` to print the exact host URL.

### Health check

| | |
|---|---|
| `protomock healthcheck` | Exit `0` when the HTTP and gRPC servers answer. It reads the same flags and env vars as the server. The Docker image uses it as its `HEALTHCHECK`. |
| `GET /protomock-ui/api/health` | `200 {"status":"ok", "mocks":…, "problems":…}`, or `503` in strict mode while any mock file fails to load |

---

## 🛠 Development

```bash
make test               # unit tests
make test-integration   # integration tests (real servers on random ports)
make ci                 # everything CI runs: tidy, vet, lint, unit + integration with -race
```

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for the architecture, how a request flows through the code, the test layout (unit, integration, snapshot, benchmark), and how to work on the UI.

---

## 📌 License

MIT: free to use and modify.
