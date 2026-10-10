package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const greeterProto = `syntax = "proto3";
package greeter;
import "google/protobuf/empty.proto";

message HelloRequest { string user_id = 1; }
message HelloReply { string message = 1; }

service Greeter {
  rpc Hello (HelloRequest) returns (HelloReply);
  rpc Ping (google.protobuf.Empty) returns (google.protobuf.Empty);
}
`

// writeTree creates files (slash-separated relative path -> content) under a temp root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadAll(t *testing.T) {
	root := writeTree(t, map[string]string{
		"http/greeter/greeter.proto":      greeterProto,
		"http/greeter/stubs/proto.json":   `{"request":{"method":"GET","url":"/p"},"response":{"message":"greeter.HelloReply","body":{"message":"hi"},"proto":true}}`,
		"http/greeter/stubs/plain.json":   `{"request":{"method":"GET","url":"/plain"},"response":{"body":{"ok":true}}}`,
		"http/greeter/stubs/notes.txt":    `ignored: not .json`,
		"http/team/users/stubs/list.json": `{"request":{"method":"GET","url":"/users"},"response":{"status":200,"body":[]}}`,
		"grpc/greeter/greeter.proto":      greeterProto,
		"grpc/greeter/stubs/hello.json":   `{"request":{"url":"/greeter.Greeter/Hello"},"response":{"body":{"message":"hi"}}}`,
	})

	set, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Problems) > 0 {
		t.Fatalf("unexpected problems: %v", set.Problems)
	}

	var names []string
	for _, m := range set.Mocks {
		names = append(names, string(m.Kind)+":"+m.Name)
	}
	if got, want := strings.Join(names, ","), "http:greeter,http:team/users,grpc:greeter"; got != want {
		t.Errorf("mocks = %s, want %s", got, want)
	}
	if len(set.HTTP) != 3 || len(set.GRPC) != 1 {
		t.Fatalf("got %d HTTP / %d gRPC routes, want 3 / 1", len(set.HTTP), len(set.GRPC))
	}

	for _, r := range set.HTTP {
		if r.URL == "/plain" && r.Status != 200 {
			t.Errorf("plain stub status = %d, want default 200", r.Status)
		}
		if r.URL == "/p" && !r.ProtoEncoded {
			t.Error("proto stub should be protobuf-encoded")
		}
	}
	if g := set.GRPC[0]; g.RequestDesc == nil || g.MessageDesc.FullName() != "greeter.HelloReply" || g.Mock != "greeter" {
		t.Errorf("gRPC route not resolved from proto: request=%v message=%v mock=%q", g.RequestDesc, g.MessageDesc, g.Mock)
	}
}

func TestLoadAll_MissingFolders(t *testing.T) {
	set, err := LoadAll(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing mocks dir should not be an error: %v", err)
	}
	if len(set.Mocks) != 0 {
		t.Errorf("got %d mocks, want 0", len(set.Mocks))
	}
}

func TestLoadAll_TwoProtosLoadStubsOnce(t *testing.T) {
	// Previously each .proto loaded the folder's stubs again, duplicating routes.
	root := writeTree(t, map[string]string{
		"grpc/svc/a.proto":       "syntax = \"proto3\"; package a; message Req {} message Resp { string v = 1; }",
		"grpc/svc/b.proto":       "syntax = \"proto3\"; package b; import \"a.proto\"; service S { rpc Call (a.Req) returns (a.Resp); }",
		"grpc/svc/stubs/ok.json": `{"request":{"url":"/b.S/Call"},"response":{"body":{"v":"x"}}}`,
	})
	set, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.GRPC) != 1 {
		t.Errorf("got %d gRPC routes, want 1 (problems: %v)", len(set.GRPC), set.Problems)
	}
}

func TestLoadAll_Problems(t *testing.T) {
	root := writeTree(t, map[string]string{
		"http/broken/broken.proto":      `syntax = "proto3"; message {`,
		"http/broken/stubs/proto.json":  `{"request":{"method":"GET","url":"/p"},"response":{"message":"X","proto":true}}`,
		"http/broken/stubs/plain.json":  `{"request":{"method":"GET","url":"/still-works"},"response":{"body":{}}}`,
		"http/bad/stubs/not-json.json":  `{nope`,
		"http/bad/stubs/no-method.json": `{"request":{"url":"/x"},"response":{}}`,
		"http/bad/stubs/bad-regex.json": `{"request":{"method":"GET","url":"/x/[0-9"},"response":{}}`,
	})
	set, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(set.HTTP) != 1 || set.HTTP[0].URL != "/still-works" {
		t.Errorf("plain JSON stub should still load next to a broken proto; routes = %v", set.HTTP)
	}

	wantErrors := map[string]string{
		"broken":         "", // proto compile error (message text comes from the compiler)
		"proto.json":     "don't compile",
		"not-json.json":  "invalid JSON",
		"no-method.json": "request.method is required",
		"bad-regex.json": "not a valid regex",
	}
	if len(set.Problems) != len(wantErrors) {
		t.Fatalf("got %d problems, want %d: %v", len(set.Problems), len(wantErrors), set.Problems)
	}
	for _, p := range set.Problems {
		want, ok := wantErrors[filepath.Base(p.Path)]
		if !ok {
			t.Errorf("unexpected problem for %s: %s", p.Path, p.Error)
			continue
		}
		if !strings.Contains(p.Error, want) {
			t.Errorf("%s: error %q should contain %q", filepath.Base(p.Path), p.Error, want)
		}
	}
}
