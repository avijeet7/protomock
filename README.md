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
docker run -p 8085:8085 -p 8086:8086 \
  -v $(pwd)/mymocks:/app/mocks \
  avijeet7/protomock:latest
```

> Replace `$(pwd)/mymocks` with your actual mocks directory.

---

## 🧪 Testing

### 🔗 HTTP Example

```bash
curl -X POST http://localhost:8085/hello/http \
  -H "Content-Type: application/json" \
  -H "X-Test-Header: mocked" \
  -d '{"user_id": "abc123"}'
```

### 🔗 Regex HTTP Example

```bash
curl -X GET http://localhost:8085/hello/123/world
```

### 🔗 gRPC Example

```bash
grpcurl -plaintext \
  -proto mocks/grpc/hello/hello.proto \
  -d '{}' \
  localhost:8086 test.FakeService/Hello
```

---

## 📌 License

MIT — free to use and modify.
