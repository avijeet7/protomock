//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/gkampitakis/go-snaps/match"
	"github.com/gkampitakis/go-snaps/snaps"
)

// editorFixture is a small mocks folder covering every kind of mock the editor shows.
var editorFixture = map[string]string{
	"http/plain/stubs/list.json":    `{"request":{"method":"GET","url":"/users"},"response":{"body":["bob"]}}`,
	"http/plain/stubs/broken.json":  `{"request":{"url":"/no-method"},"response":{}}`,
	"http/greeter/greeter.proto":    httpGreeterProto,
	"http/greeter/stubs/hello.json": `{"priority":5,"request":{"method":"GET","url":"/hello"},"response":{"status":201,"message":"greeter.HelloResponse","body":{"message":"hi","code":7},"proto":true}}`,
	"grpc/greeter/greeter.proto":    greeterProto,
	"grpc/greeter/stubs/bob.json":   greeterStub("bob", "hi bob"),
}

type editorClient struct {
	t    *testing.T
	base string // e.g. http://127.0.0.1:1234
}

func startEditor(t *testing.T) (*editorClient, string) {
	t.Helper()
	mocks := t.TempDir()
	writeFiles(t, mocks, editorFixture)
	_, base, _ := startApp(t, mocks, 0)
	return &editorClient{t: t, base: base}, mocks
}

// do sends a request to the editor API and returns the status and raw body.
func (c *editorClient) do(method, path string, query url.Values, body string) (int, []byte) {
	c.t.Helper()
	u := c.base + "/protomock-ui/api" + path
	if query != nil {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(method, u, bytes.NewBufferString(body))
	if err != nil {
		c.t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func stubQuery(kind, mock, file string) url.Values {
	return url.Values{"kind": {kind}, "mock": {mock}, "file": {file}}
}

func decode(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("invalid JSON response: %v\n%s", err, data)
	}
	return v
}

func TestEditorAPI_ListMocks(t *testing.T) {
	c, _ := startEditor(t)
	status, body := c.do("GET", "/mocks", nil, "")
	if status != 200 {
		t.Fatalf("status = %d: %s", status, body)
	}
	// mocks_dir is a temp folder; everything else must match the snapshot exactly.
	snaps.MatchJSON(t, body, match.Any("mocks_dir"))
}

func TestEditorAPI_Template(t *testing.T) {
	c, _ := startEditor(t)
	status, body := c.do("GET", "/template", url.Values{"kind": {"http"}, "mock": {"greeter"}, "message": {"greeter.HelloResponse"}}, "")
	if status != 200 {
		t.Fatalf("status = %d: %s", status, body)
	}
	snaps.MatchJSON(t, body)
}

func TestEditorAPI_SaveStubIsServedImmediately(t *testing.T) {
	c, mocks := startEditor(t)
	stub := `{"request":{"method":"POST","url":"/users"},"response":{"status":201,"body":{"id":2}}}`

	status, body := c.do("PUT", "/stubs", stubQuery("http", "plain", "create.json"), stub)
	if status != 200 {
		t.Fatalf("save: status = %d: %s", status, body)
	}
	if saved, _ := os.ReadFile(filepath.Join(mocks, "http/plain/stubs/create.json")); string(saved) != stub {
		t.Errorf("file content = %s", saved)
	}
	resp, err := http.Post(c.base+"/users", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("saved stub not served without reload: status %d", resp.StatusCode)
	}
}

func TestEditorAPI_RejectsInvalidStubsAndNames(t *testing.T) {
	c, mocks := startEditor(t)
	tests := []struct {
		name       string
		query      url.Values
		body       string
		wantStatus int
	}{
		{"invalid stub", stubQuery("http", "plain", "bad.json"), `{"request":{"url":"/x"}}`, 400},
		{"not JSON", stubQuery("http", "plain", "bad.json"), `{nope`, 400},
		{"unknown message", stubQuery("http", "greeter", "bad.json"), `{"request":{"method":"GET","url":"/x"},"response":{"proto":true,"message":"greeter.Nope"}}`, 400},
		{"path traversal in mock", stubQuery("http", "../../etc", "bad.json"), `{}`, 400},
		{"path traversal in file", stubQuery("http", "plain", "../bad.json"), `{}`, 400},
		{"unknown kind", stubQuery("ftp", "plain", "bad.json"), `{}`, 400},
		{"missing mock", stubQuery("http", "nope", "bad.json"), `{"request":{"method":"GET","url":"/x"}}`, 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := c.do("PUT", "/stubs", tt.query, tt.body)
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d: %s", status, tt.wantStatus, body)
			}
			if decode(t, body)["error"] == "" {
				t.Error("expected an error message")
			}
		})
	}
	if files, _ := filepath.Glob(filepath.Join(mocks, "http/*/stubs/bad.json")); len(files) > 0 {
		t.Errorf("rejected stubs were written: %v", files)
	}
}

func TestEditorAPI_Validate(t *testing.T) {
	c, _ := startEditor(t)
	for _, tt := range []struct {
		content string
		valid   bool
	}{
		{`{"request":{"method":"GET","url":"/ok"}}`, true},
		{`{"request":{"url":"/ok"}}`, false},
	} {
		req, _ := json.Marshal(map[string]string{"kind": "http", "mock": "plain", "content": tt.content})
		_, body := c.do("POST", "/validate", nil, string(req))
		if got := decode(t, body)["valid"]; got != tt.valid {
			t.Errorf("validate(%s) = %s, want valid=%v", tt.content, body, tt.valid)
		}
	}
}

func TestEditorAPI_DeleteStub(t *testing.T) {
	c, _ := startEditor(t)
	if status, body := c.do("DELETE", "/stubs", stubQuery("http", "plain", "list.json"), ""); status != 200 {
		t.Fatalf("delete: %d %s", status, body)
	}
	if code, _ := get(t, c.base+"/users"); code != 404 {
		t.Errorf("deleted stub still served: %d", code)
	}
	if status, _ := c.do("DELETE", "/stubs", stubQuery("http", "plain", "list.json"), ""); status != 404 {
		t.Errorf("deleting a missing stub: status %d, want 404", status)
	}
}

func TestEditorAPI_CreateGRPCMockAndTryIt(t *testing.T) {
	c, _ := startEditor(t)
	proto := `syntax = "proto3";
package shop;
message GetItemRequest { string id = 1; }
message Item { string id = 1; string name = 2; }
service Shop { rpc GetItem (GetItemRequest) returns (Item); }
`
	req, _ := json.Marshal(map[string]string{"kind": "grpc", "name": "shop", "proto": proto})
	if status, body := c.do("POST", "/mocks", nil, string(req)); status != 200 {
		t.Fatalf("create mock: %d %s", status, body)
	}
	if status, _ := c.do("POST", "/mocks", nil, string(req)); status != 409 {
		t.Errorf("creating an existing mock: status %d, want 409", status)
	}

	stub := `{"request":{"url":"/shop.Shop/GetItem","body":{"id":"42"}},"response":{"headers":{"x-shop":"yes"},"body":{"id":"42","name":"Lamp"}}}`
	if status, body := c.do("PUT", "/stubs", stubQuery("grpc", "shop", "item_42.json"), stub); status != 200 {
		t.Fatalf("save stub: %d %s", status, body)
	}

	try := func(id string) map[string]any {
		body, _ := json.Marshal(map[string]any{"method": "/shop.Shop/GetItem", "body": map[string]string{"id": id}})
		status, resp := c.do("POST", "/try/grpc", nil, string(body))
		if status != 200 {
			t.Fatalf("try: %d %s", status, resp)
		}
		return decode(t, resp)
	}

	ok := try("42")
	if ok["code"] != "OK" || ok["headers"].(map[string]any)["x-shop"] != "yes" {
		t.Errorf("try(42) = %v", ok)
	}
	if name := ok["body"].(map[string]any)["name"]; name != "Lamp" {
		t.Errorf("reply name = %v, want Lamp", name)
	}
	if miss := try("7"); miss["code"] != "NotFound" {
		t.Errorf("try(7) code = %v, want NotFound", miss["code"])
	}
}

func TestEditorAPI_TryHTTPDecodesProtobuf(t *testing.T) {
	c, _ := startEditor(t)
	req, _ := json.Marshal(map[string]string{
		"method": "GET", "path": "/hello", "kind": "http", "mock": "greeter", "message": "greeter.HelloResponse",
	})
	status, body := c.do("POST", "/try/http", nil, string(req))
	if status != 200 {
		t.Fatalf("try: %d %s", status, body)
	}
	resp := decode(t, body)
	if resp["status"] != float64(201) {
		t.Errorf("status = %v, want 201", resp["status"])
	}
	decoded, _ := resp["body_json"].(map[string]any)
	if decoded["message"] != "hi" || decoded["code"] != float64(7) {
		t.Errorf("decoded body = %v (decode_error: %v)", resp["body_json"], resp["decode_error"])
	}
}

func TestEditorAPI_SaveProtoMustCompile(t *testing.T) {
	c, _ := startEditor(t)
	q := url.Values{"kind": {"grpc"}, "mock": {"greeter"}, "file": {"greeter.proto"}}
	if status, body := c.do("PUT", "/protos", q, `syntax = "proto3"; message {`); status != 400 {
		t.Errorf("broken proto: status %d, want 400: %s", status, body)
	}
	// The original proto must still be served.
	status, body := c.do("GET", "/mocks", nil, "")
	if status != 200 || bytes.Contains(body, []byte(`message {`)) {
		t.Errorf("broken proto was saved")
	}
}
