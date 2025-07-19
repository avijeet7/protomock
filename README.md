# 🧪 ProtoMock: Dynamic HTTP & gRPC Mock Server

**ProtoMock** is a powerful Go-based server that dynamically mocks HTTP and gRPC endpoints using `.proto` files and JSON
stub responses. Ideal for frontend/backend teams needing quick, consistent mocks for integration testing or local
development.

---

## 📦 Features

- ✅ Serve both **HTTP** and **gRPC** endpoints from a single server
- 🔁 Hot-swappable mock structure using `mocks/http` and `mocks/grpc`
- 🧠 JSON-based mock responses auto-marshaled to Protobuf
- 📜 Auto registers all proto-defined message types
- 💡 Supports dynamic path registration from stubs
- 🧰 Includes reflection for `grpcurl` debugging support
- 📁 Clean modular codebase — easy to extend

---

## 📁 Project Structure

```text
protomock/
├── cmd/
│   └── server/
│       └── main.go                  # Starts both HTTP and gRPC servers
├── internal/
│   ├── grpcserver/
│   │   └── grpc_server.go           # gRPC server setup
│   ├── httpserver/
│   │   └── http_server.go           # HTTP server setup
│   ├── loader/
│   │   └── loader.go                # Stub and proto loader
│   └── models/
│       └── stub.go                  # Route and stub definitions
├── mocks/
│   ├── http/
│   │   └── <service>/               # Proto + stub folder for HTTP
│   └── grpc/
│       └── <service>/               # Proto + stub folder for gRPC
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

Once pushed to DockerHub:

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -v $(pwd)/mymocks:/app/mocks \
  avijeet7/protomock:latest
```

> Replace `$(pwd)/mymocks` with your local mocks directory.

---

## 🧪 Testing

### 🔗 HTTP Test

```bash
curl http://localhost:8080/hello \
  -H "Content-Type: application/x-protobuf"
```

---

### 🔗 gRPC Test

```bash
grpcurl -plaintext \
  -proto mocks/grpc/greeter/greeter.proto \
  -d '{"name": "Avijeet"}' \
  localhost:9090 greeter.Greeter/SayHello
```

---

## ⚙️ Configuration Details

**Protos go under:**

- `mocks/http/<service>/` for HTTP
- `mocks/grpc/<service>/` for gRPC

Each service folder should have:

- One or more `.proto` files
- A `stubs/` folder with JSON stub files

Stub files look like:

```json
{
  "url": "/hello",
  "status": 200,
  "message": "test.TestResponse",
  "response": {
    "message": "Hello there!",
    "code": 123
  }
}
```

---

## 🧼 .gitignore

The project includes:

```gitignore
mocks/
*.pb.go
*.pb.json
vendor/
*.exe
*.out
dist/
```

---

## 📌 License

MIT — free to use and modify.
