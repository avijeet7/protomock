//go:build integration

package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/avijeet7/protomock/internal/httpserver"
	"github.com/avijeet7/protomock/internal/loader"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

const httpGreeterProto = `syntax = "proto3";
package greeter;

message HelloResponse {
  string message = 1;
  int32 code = 2;
  repeated string tags = 3;
}
`

// startHTTPMocks writes an HTTP mock folder (proto + stubs) and serves it with the real loader and router.
func startHTTPMocks(t *testing.T, files map[string]string) *httptest.Server {
	t.Helper()
	root := filepath.Join(t.TempDir(), "mocks")
	writeFiles(t, filepath.Join(root, "http"), files)

	set, err := loader.LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Problems) > 0 {
		t.Fatalf("unexpected load problems: %v", set.Problems)
	}

	srv := httptest.NewServer(httpserver.NewRouter(set.HTTP, http.NotFoundHandler()))
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTP_ProtobufResponse(t *testing.T) {
	srv := startHTTPMocks(t, map[string]string{
		"greeter/greeter.proto": httpGreeterProto,
		"greeter/stubs/hello.json": `{
  "request": {"method": "GET", "url": "/hello", "headers": {"X-Test-Header": "a"}},
  "response": {"status": 201, "message": "greeter.HelloResponse",
               "body": {"message": "hi", "code": 42, "tags": ["x", "y"]}, "proto": true}
}`,
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/hello", nil)
	req.Header.Set("X-Test-Header", "a")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-protobuf" {
		t.Errorf("Content-Type = %q, want application/x-protobuf", ct)
	}

	md := compileProto(t, "greeter.proto", httpGreeterProto).Messages().ByName("HelloResponse")
	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(data, msg); err != nil {
		t.Fatalf("response is not a valid greeter.HelloResponse: %v", err)
	}
	if got := msg.Get(md.Fields().ByName("message")).String(); got != "hi" {
		t.Errorf("message = %q, want %q", got, "hi")
	}
	if got := msg.Get(md.Fields().ByName("code")).Int(); got != 42 {
		t.Errorf("code = %d, want 42", got)
	}
	if got := msg.Get(md.Fields().ByName("tags")).List().Len(); got != 2 {
		t.Errorf("len(tags) = %d, want 2", got)
	}
}

func TestHTTP_ProtoAndJSONStubsInSameFolder(t *testing.T) {
	srv := startHTTPMocks(t, map[string]string{
		"greeter/greeter.proto": httpGreeterProto,
		"greeter/stubs/proto.json": `{
  "request": {"method": "GET", "url": "/proto"},
  "response": {"status": 200, "message": "greeter.HelloResponse", "body": {"message": "p"}, "proto": true}
}`,
		"greeter/stubs/plain.json": `{
  "request": {"method": "GET", "url": "/plain"},
  "response": {"status": 200, "body": {"message": "plain"}}
}`,
		"plain_only/stubs/users.json": `{
  "request": {"method": "POST", "url": "/users", "body": {"name": "bob"}},
  "response": {"status": 201, "body": {"id": 1}}
}`,
	})

	tests := []struct {
		method, path, body string
		wantStatus         int
		wantType, wantBody string
	}{
		{"GET", "/proto", "", 200, "application/x-protobuf", ""},
		{"GET", "/plain", "", 200, "application/json", `{"message":"plain"}`},
		{"POST", "/users", `{"name":"bob","age":3}`, 201, "application/json", `{"id":1}`},
		{"POST", "/users", `{"name":"alice"}`, 404, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path+" "+tt.body, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, srv.URL+tt.path, strings.NewReader(tt.body))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			data, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if tt.wantType != "" && resp.Header.Get("Content-Type") != tt.wantType {
				t.Errorf("Content-Type = %q, want %q", resp.Header.Get("Content-Type"), tt.wantType)
			}
			if tt.wantBody != "" && strings.Join(strings.Fields(string(data)), "") != tt.wantBody {
				t.Errorf("body = %s, want %s", data, tt.wantBody)
			}
		})
	}
}
