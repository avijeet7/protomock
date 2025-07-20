# 🧪 ProtoMock: Dynamic HTTP & gRPC Mock Server

**ProtoMock** is a powerful Go-based server that dynamically mocks HTTP and gRPC endpoints using `.proto` files and JSON stub responses. It supports advanced matching and flexible configuration, making it ideal for integration testing, sandbox environments, and frontend/backend development workflows.

---

## 📦 Features

- ✅ Serve both **HTTP** and **gRPC** endpoints from a single mock server
- 🔁 Automatically load `.proto` + `stub.json` pairs from `mocks/` folders
- ⚡ Supports **dynamic path + message type** mapping from stubs
- 🧠 JSON stub bodies auto-marshaled to **Protobuf**
- 🔍 Header & body request **matching for HTTP and gRPC**
- 🤖 Supports **content-type-based response**: Protobuf or JSON
- 🛠 gRPC reflection for `grpcurl` debugging
- 🔐 Optional matching for headers and partial body (WireMock style)
- 🧰 Clean, extensible codebase with clear modular separation

---

## 📁 Project Structure

```text
protomock/
├── cmd/
│   └── server/
│       └── main.go                  # Starts both HTTP and gRPC servers
├── internal/
│   ├── grpcserver/
│   │   ├── server.go                # gRPC server bootstrap
│   │   ├── handler.go               # gRPC request matching and response
│   │   └── utils.go                 # Normalization helpers
│   ├── httpserver/
│   │   ├── server.go                # HTTP route bootstrap
│   │   ├── handler.go               # HTTP matching logic
│   │   └── matcher.go               # Header & body comparison logic
│   ├── loader/
│   │   ├── loader.go                # Walks folders for mocks
│   │   ├── parser.go                # Parses proto + stubs
│   │   └── message.go               # Finds proto message definitions
│   └── models/
│       ├── route.go                 # Runtime route representation
│       └── stub.go                  # Stub input definition
├── mocks/
│   ├── http/
│   │   └── <service>/               # Proto + stubs for HTTP
│   └── grpc/
│       └── <service>/               # Proto + stubs for gRPC
└── docker-compose.yml
```

---

## 🚀 Running the Project

### ▶️ For Local Development

```bash
docker-compose up --build
```

Uses the bundled `./mocks` folder mounted into the container.

---

### ▶️ From DockerHub (User Usage)

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -v $(pwd)/mymocks:/app/mocks \
  avijeet7/protomock:latest
```

> Replace `$(pwd)/mymocks` with your actual mocks directory.

---

## 🧪 Testing

### 🔗 HTTP Example

```bash
curl -X POST http://localhost:8080/hello/http \
  -H "Content-Type: application/json" \
  -H "X-Test-Header: mocked" \
  -d '{"user_id": "abc123"}'
```

### 🔗 gRPC Example

```bash
grpcurl -plaintext \
  -proto mocks/grpc/hello/hello.proto \
  -d '{}' \
  localhost:9090 test.FakeService/Hello
```

---

## ⚙️ Stub Format

**HTTP or gRPC stub file:**

```json
{
  "request": {
    "method": "POST",
    "url": "/hello/http",
    "headers": {
      "X-Test-Header": "mocked"
    },
    "body": {
      "user_id": "abc123"
    }
  },
  "response": {
    "status": 200,
    "message": "test.TestResponse",
    "body": {
      "message": "Hello from ProtoMock!",
      "code": 42
    },
    "proto": true
  }
}
```

---

## 📌 License

MIT — free to use and modify.
