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
- 🛠 gRPC reflection for `grpcurl` debugging
- 🔐 Optional header and partial body matching (WireMock style)
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

## 📁 Project Structure

```text
protomock/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── grpcserver/
│   │   ├── server.go
│   │   ├── handler.go
│   │   └── utils.go
│   ├── httpserver/
│   │   ├── server.go
│   │   ├── handler.go
│   │   └── matcher.go
│   ├── loader/
│   │   ├── loader.go
│   │   ├── parser.go
│   │   └── message.go
│   └── models/
│       ├── route.go
│       └── stub.go
├── mocks/
│   ├── http/
│   │   └── <service>/...
│   └── grpc/
│       └── <service>/...
└── docker-compose.yml
```

---

## 🚀 Running the Project

### ▶️ For Local Development

```bash
docker-compose up --build
```

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

## 📌 License

MIT — free to use and modify.
