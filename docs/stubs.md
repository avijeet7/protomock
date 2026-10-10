# Stub Reference

A **stub** is a JSON file in a mock's `stubs/` folder. It says *"when a request looks like this, reply with that"*. This page covers every field and rule, with an example for each case.

- [Mock folders](#mock-folders)
- [Stub fields](#stub-fields)
- [Matching rules](#matching-rules)
- [HTTP examples](#http-examples)
- [gRPC examples](#grpc-examples)
- [Validation errors](#validation-errors)

---

## Mock folders

```text
mocks/
├── http/<mock>/          HTTP mocks
│   ├── *.proto           optional: only needed for protobuf responses
│   └── stubs/*.json
└── grpc/<mock>/          gRPC mocks
    ├── *.proto           service + message definitions
    └── stubs/*.json
```

- **A mock folder** is any folder under `http/` or `grpc/` that contains a `stubs/` folder or `.proto` files.
  - Folders can be nested to group mocks, e.g. `http/payments/refunds/`.
  - Each mock's name is its path, e.g. `payments/refunds`.
- **All `.proto` files directly in a mock folder are compiled together.**
  - Imports resolve relative to the mock folder, so `import "common/types.proto";` works.
  - Well-known types (`google/protobuf/*.proto`, e.g. `Timestamp`, `Empty`) are built in.
- **Only `stubs/*.json` files are loaded as stubs.** Other files are ignored.
- **One bad file never stops the server.** A stub or `.proto` that fails to load is skipped and reported:
  - in the startup log (`⚠️ Skipped ...`),
  - in the UI (the **⚠ problems** button and red markers).

## Stub fields

```jsonc
{
  "priority": 0,               // optional: higher is tried first (default 0)
  "request": {
    "method": "GET",           // HTTP only, required for HTTP stubs
    "url": "/users",           // required: HTTP path or regex; gRPC "/package.Service/Method"
    "headers": { "X-Tenant": "acme" },   // optional: headers (HTTP) / metadata (gRPC) that must be present
    "body": { "name": "bob" }  // optional: partial JSON the request body must contain
  },
  "response": {
    "status": 200,             // HTTP only: status code (default 200)
    "headers": { "X-Request-Id": "1" },  // optional: HTTP response headers / gRPC response metadata
    "proto": false,            // HTTP only: true = encode body as protobuf (needs "message")
    "message": "pkg.Reply",    // protobuf message type (HTTP with proto:true; gRPC: defaults to the method's output)
    "body": { "id": 1 },       // the response body as JSON (any JSON value for plain HTTP)
    "delay_ms": 0              // optional: wait this long before replying
  }
}
```

| Field | HTTP | gRPC | Notes |
|---|---|---|---|
| `priority` | ✓ | ✓ | Integer, may be negative. Default `0`. |
| `request.method` | **required** | ignored | Any HTTP method, case-insensitive. |
| `request.url` | **required** | **required** | HTTP: must start with `/`. gRPC: `/package.Service/Method` (the leading `/` is optional). |
| `request.headers` | ✓ | ✓ (metadata) | Exact values. HTTP header names are case-insensitive; gRPC metadata keys are lowercase. |
| `request.body` | ✓ | ✓ | Partial JSON match, see below. gRPC needs the method in the `.proto`. |
| `response.status` | ✓ | ignored | `100`–`999`, default `200`. |
| `response.headers` | ✓ | ✓ (metadata) | May override `Content-Type`. |
| `response.proto` | ✓ | always on | HTTP: `false` = JSON response, `true` = protobuf. |
| `response.message` | with `proto` | optional | Full message name, e.g. `greeter.HelloReply`. gRPC: must match the method's output type. |
| `response.body` | ✓ | ✓ | For protobuf, the JSON form of the message (see below). |
| `response.delay_ms` | ✓ | ✓ | Stops early if the client gives up (gRPC returns `DeadlineExceeded` to the client). |

Unknown keys are ignored, so you can keep notes in a stub, e.g. `"_comment": "..."`. The UI keeps them when you save.

### Protobuf bodies

Protobuf bodies are written as **proto3 JSON**:
- Field names can be the `.proto` names (`user_id`) or camelCase (`userId`).
- 64-bit integers are strings (`"revenue": "100"`).
- Enums are names (`"status": "ACTIVE"`).
- Timestamps are RFC 3339 strings (`"2024-01-01T00:00:00Z"`).
- Unknown fields are an error, which catches typos.

In the UI, **Insert template** writes a body with every field of the message filled in.

---

## Matching rules

A request is answered by the **first** stub that matches **all** of its rules:

1. **Method** (HTTP): equal, case-insensitive.
2. **URL**:
   - **Exact:** the path must be equal. The query string is ignored.
   - **Regex:** any URL containing `* + ? ( ) [ ] { }` is a regular expression, **anchored**, so the whole path must match. For example, `/users/[0-9]+` matches `/users/42` but not `/users/42/orders`.
   - **gRPC:** the full method name, `/package.Service/Method`.
3. **Headers / metadata:** every listed header must be present with exactly that value. Extra request headers are ignored.
4. **Body:** a **partial JSON** match.
   - Every field in the stub's `request.body` must be in the request with the same value, and nested objects are checked the same way.
   - Extra fields in the request are ignored.
   - Arrays and plain values must be **equal**, so array order matters.
   - A stub without `request.body` matches any body. An invalid JSON request never matches a stub that has a body.
   - **gRPC:** the request is converted to JSON with every field included, so a stub can match zero values like `{"count": 0}` or `{"name": ""}`.

**Which stub wins:**
- Stubs are tried in order of `priority`, **highest first**.
- Stubs with the same priority are tried in mock-name order, then file-name order.
- HTTP stubs with an **exact URL** are always tried before **regex** stubs.

**When nothing matches:**

| | Response |
|---|---|
| HTTP | `404 Not Found` |
| gRPC, stubs exist for the method | `NotFound`: *"none of the N stubs … matched"* |
| gRPC, no stubs for the method | `Unimplemented` |

---

## HTTP examples

### JSON response (no `.proto` needed)

```json
{
  "request": { "method": "GET", "url": "/api/v1/users" },
  "response": {
    "status": 200,
    "body": [ { "id": 1, "name": "John" }, { "id": 2, "name": "Jane" } ]
  }
}
```

The response is sent with `Content-Type: application/json`.

### Regex URL

```json
{
  "request": { "method": "GET", "url": "/users/[0-9]+/orders" },
  "response": { "body": { "orders": [] } }
}
```

Matches `/users/1/orders` and `/users/987/orders`, but not `/users/abc/orders`.

### Match on headers

```json
{
  "request": {
    "method": "GET",
    "url": "/profile",
    "headers": { "Authorization": "Bearer test-token" }
  },
  "response": { "body": { "name": "Test User" } }
}
```

### Match on part of the body

```json
{
  "request": {
    "method": "POST",
    "url": "/login",
    "body": { "user": "bob", "options": { "remember": true } }
  },
  "response": { "status": 200, "body": { "token": "abc" } }
}
```

Matches `{"user":"bob","password":"x","options":{"remember":true,"device":"ios"}}`. Extra fields are fine.

### Overlapping stubs: priority and a fallback

`admin.json`, tried first:
```json
{
  "priority": 10,
  "request": { "method": "POST", "url": "/login", "body": { "user": "admin" } },
  "response": { "body": { "role": "admin" } }
}
```

`fallback.json`, tried last, answers every other `POST /login`:
```json
{
  "priority": -1,
  "request": { "method": "POST", "url": "/login" },
  "response": { "status": 401, "body": { "error": "invalid credentials" } }
}
```

### Errors, custom headers and slow responses

```json
{
  "request": { "method": "GET", "url": "/flaky" },
  "response": {
    "status": 503,
    "headers": { "Retry-After": "5", "Content-Type": "application/problem+json" },
    "body": { "title": "Service Unavailable" },
    "delay_ms": 1500
  }
}
```

### Protobuf response

Put a `.proto` in the HTTP mock folder, e.g. `mocks/http/greeter/greeter.proto`:

```proto
syntax = "proto3";
package greeter;
message HelloResponse { string message = 1; int32 code = 2; }
```

```json
{
  "request": { "method": "GET", "url": "/hello" },
  "response": {
    "proto": true,
    "message": "greeter.HelloResponse",
    "body": { "message": "Hello!", "code": 42 }
  }
}
```

The response is protobuf-encoded with `Content-Type: application/x-protobuf`.

---

## gRPC examples

All examples use this `mocks/grpc/shop/shop.proto`:

```proto
syntax = "proto3";
package shop;

message GetItemRequest { string id = 1; int32 version = 2; }
message Item { string id = 1; string name = 2; int64 price_cents = 3; }

service Shop {
  rpc GetItem (GetItemRequest) returns (Item);
}
```

### Any request

```json
{
  "request": { "url": "/shop.Shop/GetItem" },
  "response": { "body": { "id": "1", "name": "Default item", "price_cents": "999" } }
}
```

`response.message` isn't needed: it defaults to the method's output type, `shop.Item`.

### Match on request fields

```json
{
  "request": { "url": "/shop.Shop/GetItem", "body": { "id": "42" } },
  "response": { "body": { "id": "42", "name": "Lamp", "price_cents": "4999" } }
}
```

camelCase works too: `{ "priceCents": "4999" }`.

### Match on metadata and return metadata

```json
{
  "request": {
    "url": "/shop.Shop/GetItem",
    "headers": { "x-tenant": "acme" }
  },
  "response": {
    "headers": { "x-served-by": "protomock" },
    "body": { "id": "1", "name": "Acme item" }
  }
}
```

### Slow responses (test client deadlines)

```json
{
  "request": { "url": "/shop.Shop/GetItem", "body": { "id": "slow" } },
  "response": { "delay_ms": 3000, "body": { "id": "slow" } }
}
```

A client with a deadline shorter than 3 s gets `DeadlineExceeded`.

### Calling gRPC mocks

Reflection is enabled, so `grpcurl` needs no `.proto`:

```bash
grpcurl -plaintext localhost:8086 list
grpcurl -plaintext localhost:8086 describe shop.Shop
grpcurl -plaintext -H "x-tenant: acme" -d '{"id": "42"}' localhost:8086 shop.Shop/GetItem
```

> Mocks are designed for **unary** methods; streaming methods are not supported.

---

## Validation errors

These errors show up in the UI as you type, and in the startup log for files on disk:

| Error | Fix |
|---|---|
| `request.url is required` | Add `request.url`. |
| `request.method is required for HTTP stubs` | Add e.g. `"method": "GET"`. |
| `request.url must start with /` | HTTP paths start with `/`. |
| `request.url is not a valid regex` | Fix the pattern, or escape the special characters. |
| `response.message ... is required when response.proto is true` | Name the message type, e.g. `greeter.HelloResponse`. |
| `message type "X" not found in the mock's .proto files` | Check the package prefix and spelling; the type must be in this mock's protos or their imports. |
| `response.body doesn't match X: unknown field "y"` | The body has a field the message doesn't define. |
| `request.url must be a gRPC method path like "/package.Service/Method"` | Use the full service name, including its package. |
| `method ... not found in the mock's .proto files (needed for request.body matching)` | Add the service to the mock's `.proto`, or remove `request.body`. |
| `response.message X doesn't match the method's output type Y` | Remove `response.message`, or set it to the output type. |
| `the mock's .proto files don't compile` | Fix the `.proto` (the exact compiler error is listed in Problems). |
