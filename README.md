# 🧪 ProtoMock: Dynamic HTTP & gRPC Mock Server

**ProtoMock** is a powerful Go-based server that dynamically mocks HTTP and gRPC endpoints using `.proto` files and JSON stub responses. It supports advanced matching and flexible configuration, making it ideal for integration testing, sandbox environments, and frontend/backend development workflows.

---

## 📦 Features

- ✅ Serve both **HTTP** and **gRPC** endpoints from a single mock server
- 🔁 Automatically load `.proto` + `stub.json` pairs from `mocks/` folders
- ⚡ Supports **dynamic path + message type** mapping from stubs
- 🧠 JSON stub bodies auto-marshaled to **Protobuf**
- 🔍 Header & body request **matching for HTTP and gRPC**
- 🤖 Supports **Protobuf or JSON** response encoding
- 🎯 Regex URL matching for HTTP endpoints
- 🛠 gRPC reflection for `grpcurl` debugging
- 🔐 Optional header and partial body matching (WireMock style)
- 💻 Web UI to view configured mocks
- 📝 Supports plain JSON mocks for http endpoints
- 🧰 Clean, extensible codebase with modular separation

---

## 🚀 How to Use

1. **Start the server**
   You can run using Docker or from source.

2. **Create your mocks folder**
   Your `mocks` directory should follow this structure:

   ```text
   mocks/
   ├── http/
   │   └── <mockName>/
   │       ├── service.proto
   │       └── stubs/
   │           ├── stub1.json
   │           └── stub2.json
   └── grpc/
       └── <mockName>/
           ├── service.proto
           └── stubs/
               ├── stub1.json
               └── stub2.json
   ```

3. **Stub Format**
   Each stub JSON must define:
   ```json
   {
     "request": {
       "method": "POST",
       "url": "/test.FakeService/Hello",
       "headers": {
         "X-Test-Header": "mocked1"
       },
       "body": {
         "user_id": "abc123"
       }
     },
     "response": {
       "status": 200,
       "message": "test.TestResponse",
       "body": {
         "message": "Hello from fake service",
         "code": 200
       },
       "proto": true
     }
   }
   ```

---

## ✨ Web UI

ProtoMock includes a web interface to view all configured mock endpoints. Simply navigate to `http://localhost:8085/protomock-ui` in your browser to see a list of all loaded HTTP and gRPC stubs.

## 📝 Plain JSON Mocks

For HTTP endpoints, you can now use plain JSON mocks without a `.proto` file. This is useful for mocking simple REST APIs. To use this feature, create a `stubs` directory inside `mocks/http/<mockName>/` and place your JSON files there. The server will automatically pick them up.

In the stub file, you can omit the `response.message` and `response.proto` fields. For example:

```json
{
  "request": {
    "method": "GET",
    "url": "/api/v1/users"
  },
  "response": {
    "status": 200,
    "headers": {
      "Content-Type": "application/json"
    },
    "body": [
      {
        "id": 1,
        "name": "John Doe"
      },
      {
        "id": 2,
        "name": "Jane Doe"
      }
    ]
  }
}
```

## 🎯 Regex URL Matching

ProtoMock supports regex matching for HTTP endpoint URLs. This allows you to match dynamic URLs with a single stub.

To use this feature, simply provide a valid regex in the `url` field of your stub file. For example:

```json
{
  "request": {
    "method": "GET",
    "url": "/hello/[0-9]+/world"
  },
  "response": {
    "status": 200,
    "body": {
      "message": "Hello from regex world"
    }
  }
}
```

This stub will match any URL that starts with `/hello/`, followed by one or more digits, and ends with `/world`. For example, it will match `/hello/123/world` and `/hello/456/world`.

---

## 📁 Project Structure

```text
protomock/
├── cmd/server/            # main: load config + mocks, start servers
├── internal/
│   ├── config/            # flags / env vars / defaults
│   ├── loader/            # reads mocks/ (proto + JSON stubs) into routes
│   ├── models/            # Stub (file format) and Route (loaded mock)
│   ├── match/             # shared partial JSON body matching
│   ├── httpserver/        # HTTP router, matching, responses
│   ├── grpcserver/        # gRPC catch-all handler and matching
│   └── web/               # UI handler, templates, static assets
├── test/
│   ├── integration/       # real loader + servers on random ports
│   └── benchmark/         # performance benchmarks
├── mocks/                 # sample mocks (http/ and grpc/)
├── Makefile
└── docker-compose.yml
```

---

## 🚀 Running the Project

### ▶️ For Local Development

```bash
make run              # or: go run ./cmd/server
```

Or with Docker:

```bash
docker compose up --build
```

### ▶️ From DockerHub (User Usage)

```bash
docker run -p 8085:8085 -p 8086:8086 \
  -v $(pwd)/mymocks:/app/mocks \
  avijeet7/protomock:latest
```

> Replace `$(pwd)/mymocks` with your actual mocks directory.

---

## ⚙️ Configuration

Every setting can be passed as a flag or an environment variable.
Precedence: **flag > environment variable > default**.

| Flag | Environment variable | Default | Description |
|---|---|---|---|
| `-http-addr` | `PROTOMOCK_HTTP_ADDR` | `:8085` | HTTP mocks and UI listen address |
| `-grpc-addr` | `PROTOMOCK_GRPC_ADDR` | `:8086` | gRPC mocks listen address |
| `-mocks-dir` | `PROTOMOCK_MOCKS_DIR` | `mocks` | Folder containing `http/` and `grpc/` mocks |
| `-web-dir` | `PROTOMOCK_WEB_DIR` | `internal/web` | UI assets folder |

```bash
go run ./cmd/server -http-addr :9085 -grpc-addr :9086
docker run -p 9085:9085 -e PROTOMOCK_HTTP_ADDR=:9085 avijeet7/protomock:latest
```

If a port is already in use, ProtoMock exits with an error instead of running half-started.

### Matching rules

- **Method + URL**: exact match, or a regex (anchored: the whole path must match).
- **Headers**: every header in the stub must be present with the same value (other headers are ignored). For gRPC these are request metadata.
- **Body**: partial match. Every field in the stub must be in the request (nested objects too); extra request fields are ignored; arrays must be equal.
  For gRPC, stub bodies may use either the `.proto` field names (`user_id`) or camelCase (`userId`).
- **Order**: stubs are tried in file-name order and the first match wins, so name more specific stubs so they sort first.
- **No match**: HTTP returns `404`; gRPC returns `NotFound` (stubs exist for the method but none matched) or `Unimplemented` (no stubs for the method).

---

## 🧪 Trying the Sample Mocks

### 🔗 HTTP Example

```bash
curl -X POST http://localhost:8085/hello/http \
  -H "Content-Type: application/json" \
  -H "X-Test-Header: mocked3" \
  -d '{"user_id": "abc123"}'
```

### 🔗 Regex HTTP Example

```bash
curl -X GET http://localhost:8085/hello/123/world
```

### 🔗 gRPC Example

```bash
grpcurl -plaintext \
  -import-path mocks/grpc/hello -proto hello.proto \
  -H "X-Test-Header: mocked1" \
  -d '{}' \
  localhost:8086 test.FakeService/Hello
```

Use `X-Test-Header: mocked2` to get the other stub's response.

---

## 🛠 Development

Requires Go (version in `go.mod`). Optional: `golangci-lint`, `grpcurl`, Docker.

| Command | What it does |
|---|---|
| `make run` | Run the server |
| `make test` | Unit tests (fast, no network) |
| `make test-integration` | Integration tests: real loader + servers on random local ports |
| `make bench` | Benchmarks |
| `make lint` | `golangci-lint` (config in `.golangci.yml`) |
| `make cover` | Unit test coverage report in the browser |
| `make ci` | Same checks as GitHub CI (tidy, vet, lint, unit + integration with `-race`) |

### Test layout

| Kind | Location | Build tag |
|---|---|---|
| Unit | next to the code: `internal/**/*_test.go` | none (default) |
| Integration | `test/integration/` | `integration` |
| Benchmark | `test/benchmark/` | `bench` |

Plain `go test ./...` runs only unit tests. Integration tests and benchmarks are excluded by their build tags.

CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs lint, unit tests, integration tests and a Docker build on every push to `main` and on every pull request.

---

## 📌 License

MIT — free to use and modify.
