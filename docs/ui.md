# Web UI Guide

Open **http://localhost:8085/protomock-ui**, or whatever `-http-addr` you configured.

The UI reads and writes the same mock files you'd edit by hand. A save is validated with the server's own rules, written to disk, and **live immediately**. Changes made on disk show up in the UI within a few seconds.

- [Layout](#layout)
- [Create a mock](#create-a-mock)
- [Create and edit stubs](#create-and-edit-stubs)
- [Try it](#try-it)
- [Protos](#protos)
- [Problems](#problems)
- [Read-only mode](#read-only-mode)
- [JSON API](#json-api)

---

## Layout

- **Top bar**:
  - the mocks folder in use,
  - **⚠ problems**, if any files failed to load,
  - **⟳ Reload**, which re-reads the folder now (otherwise it happens automatically every `-reload-interval`),
  - **? Guide**, a short built-in guide.
- **Sidebar**:
  - every mock, grouped into HTTP and gRPC, with its stubs underneath;
  - **Search** filters by URL, method, file name or mock name;
  - a red dot marks a mock or stub with an error.
- **Main area**: the selected mock's overview, or the stub editor.

The URL tracks what's open (e.g. `#/grpc/shop/~get_item.json`), so you can bookmark or share a link to a stub.

## Create a mock

**+ New mock** asks for:

| | |
|---|---|
| **Type** | **HTTP** or **gRPC** |
| **Name** | The folder under `mocks/http/` or `mocks/grpc/`. Use `/` to nest, e.g. `payments/refunds`. |
| **.proto** | Required for gRPC; prefilled with a sample service you can edit. Optional for HTTP: only needed for protobuf responses. |

The `.proto` must compile before the mock is created. The mock's page then lists:
- its stubs,
- for gRPC, every method from its services, each with a **+ Stub** button,
- for HTTP, the message types you can return as protobuf,
- its `.proto` files.

## Create and edit stubs

Start a stub with **+ New stub** on a mock page, or **+ Stub** next to a gRPC method. To change an existing stub, click it in the sidebar.

### Form view

**Request: when should this stub answer?**
- **Method + URL** (HTTP). The URL is an exact path, or a regex like `/users/[0-9]+`, and the hint under it says which one you have. For gRPC, pick the **Method** from the mock's services.
- **Priority**: higher is tried first. Use it when stubs overlap.
- **Headers / Metadata must include**: name and value pairs that must be present.
- **Body must contain**: a partial JSON match. Leave it empty to match any body. For gRPC, **Insert template** fills in the request message's fields.

**Response: what should it return?**
- **Status** (HTTP), and **Delay (ms)**.
- **Format** (HTTP): tick **Protobuf** and pick a **Message type** to return protobuf. This needs a `.proto` in the mock. For gRPC, the message type is the method's output type.
- **Response headers / metadata**.
- **Body**. **Insert template** fills it with every field of the protobuf message.

### JSON view

The raw stub file. The form and JSON views edit the same stub:
- switching between them never loses changes;
- keys the form doesn't know about (e.g. `"_comment"`) are kept.

If the JSON doesn't parse, the Form tab and **Save** are disabled until you fix it.

### Validation and saving

- **Live validation:** the status line under the toolbar re-checks the stub as you type, e.g. `✓ Valid stub` or `✗ message type "x" not found …`.
- **Save** (or **⌘/Ctrl + S**) writes the file and makes it live.
- **Rename:** change **File name** and save. The old file is removed.
- **Duplicate** opens an unsaved copy.
- **Delete** removes the file after a confirmation.
- **Unsaved changes:** you're warned before navigating away.

## Try it

**▶ Try it** opens a request panel prefilled from the stub: method, URL, headers and body.
- **HTTP:**
  - The request goes through the real mock router, so you see exactly what a client would get.
  - Protobuf responses are **decoded to JSON** using the stub's message type.
  - For regex URLs, edit the path to a concrete one before sending.
- **gRPC:**
  - A real gRPC call goes to the running server.
  - You see the status code, response metadata, the reply as JSON, and the duration.

Only **saved** changes are live. Save first to try an edit.

## Protos

Each `.proto` file is editable on the mock's page:
- **Save .proto** compiles it together with the mock's other protos first, and rejects it with the compiler error if it doesn't compile;
- **+ Add .proto file** adds another file, e.g. shared types to `import`;
- **Delete** removes a file. Stubs that use its types stop loading and show up under Problems.

## Problems

**⚠ N problems** lists every file that failed to load, with the reason. These are the same errors as the `⚠️ Skipped` lines in the server log. Fix a file in the UI or on disk, and it reloads automatically.

## Read-only mode

If the mocks folder isn't writable, the UI shows a **read-only** badge and disables editing. For example, Docker with `-v ./mocks:/app/mocks:ro`. Browsing and **Try it** still work.

---

## JSON API

The UI is built on a small JSON API, which you can also use from scripts or CI. All paths are under `/protomock-ui/api`.

Mocks are identified by query parameters, `?kind=http|grpc&mock=<name>&file=<file>`, because mock names may contain `/`.

| Method | Path | Body | Description |
|---|---|---|---|
| `GET` | `/mocks` | | Every mock with its protos, messages, gRPC methods, stubs (raw content and error) and problems, plus `writable` |
| `POST` | `/mocks` | `{"kind","name","proto_file"?,"proto"?}` | Create a mock folder, optionally with a `.proto` |
| `PUT` | `/stubs?kind&mock&file` | stub JSON | Validate and save a stub |
| `DELETE` | `/stubs?kind&mock&file` | | Delete a stub |
| `PUT` | `/protos?kind&mock&file` | `.proto` text | Compile-check and save a `.proto` |
| `DELETE` | `/protos?kind&mock&file` | | Delete a `.proto` |
| `POST` | `/validate` | `{"kind","mock","content"}` | Check a stub without saving: `{"valid": true}` or `{"valid": false, "error": "…"}` |
| `GET` | `/template?kind&mock&message` | | Example JSON body for a message type |
| `POST` | `/reload` | | Re-read the mocks folder |
| `POST` | `/try/http` | `{"method","path","headers","body","kind"?,"mock"?,"message"?}` | Send a request to the HTTP mocks |
| `POST` | `/try/grpc` | `{"method","metadata","body"}` | Call a unary gRPC mock |

- **Successful writes** return the updated `/mocks` listing.
- **Errors** return `{"error": "…"}` with a status code:
  - `400`: invalid stub, proto or name,
  - `403`: the mocks folder is read-only,
  - `404`: not found,
  - `409`: already exists.

### Example: create a mock and a stub from a script

```bash
API=http://localhost:8085/protomock-ui/api

curl -s -X POST $API/mocks -d '{"kind": "http", "name": "users"}'

curl -s -X PUT "$API/stubs?kind=http&mock=users&file=list.json" -d '{
  "request":  { "method": "GET", "url": "/users" },
  "response": { "body": [ { "id": 1, "name": "Bob" } ] }
}'

curl -s http://localhost:8085/users
```
