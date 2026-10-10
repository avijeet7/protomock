//go:build integration

// Package integration runs the real loader and servers end to end against mocks on disk.
package integration

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/loader"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const greeterProto = `syntax = "proto3";
package test;

message HelloRequest {
  string user_id = 1;
}

message HelloReply {
  string message = 1;
}

service Greeter {
  rpc Hello (HelloRequest) returns (HelloReply);
}
`

// greeterStub returns a stub that replies `reply` when the request's user_id is `userID`.
func greeterStub(userID, reply string) string {
	return greeterStubWithField("user_id", userID, reply)
}

// greeterStubWithField is greeterStub with a custom JSON key for the user id (e.g. "userId").
func greeterStubWithField(field, userID, reply string) string {
	return `{
  "request": {"url": "/test.Greeter/Hello", "body": {"` + field + `": "` + userID + `"}},
  "response": {"status": 200, "message": "test.HelloReply", "body": {"message": "` + reply + `"}, "proto": true}
}`
}

// startGreeter writes the greeter proto and stubs to a temp mocks dir, loads them with the
// real loader, and serves them on a random local port. It returns a connected client.
func startGreeter(t *testing.T, stubs map[string]string) *grpc.ClientConn {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "greeter")
	if err := os.MkdirAll(filepath.Join(dir, "stubs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "greeter.proto"), []byte(greeterProto), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range stubs {
		if err := os.WriteFile(filepath.Join(dir, "stubs", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	routes, err := loader.LoadProtoMocks(filepath.Dir(dir))
	if err != nil || len(routes) != len(stubs) {
		t.Fatalf("loaded %d routes (err=%v), want %d", len(routes), err, len(stubs))
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0") // port 0 = any free port
	if err != nil {
		t.Fatal(err)
	}
	server := grpcserver.NewGRPCServer(routes)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// callHello sends HelloRequest{user_id} to Greeter/Hello and returns the reply's message field.
func callHello(t *testing.T, conn *grpc.ClientConn, userID string) (string, error) {
	t.Helper()
	return callGreeter(t, conn, "/test.Greeter/Hello", userID)
}

// callGreeter sends HelloRequest{user_id} to any method and returns the reply's message field.
func callGreeter(t *testing.T, conn *grpc.ClientConn, method, userID string) (string, error) {
	t.Helper()

	fds, err := (&protoparse.Parser{Accessor: protoparse.FileContentsFromMap(map[string]string{
		"greeter.proto": greeterProto,
	})}).ParseFiles("greeter.proto")
	if err != nil {
		t.Fatal(err)
	}
	reqDesc, replyDesc := fds[0].FindMessage("test.HelloRequest"), fds[0].FindMessage("test.HelloReply")

	req := dynamic.NewMessage(reqDesc)
	req.SetFieldByName("user_id", userID)
	reply := dynamic.NewMessage(replyDesc)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Invoke(ctx, method, req, reply); err != nil {
		return "", err
	}
	return reply.GetFieldByName("message").(string), nil
}

func TestGRPC_BodyMatchUsesRequestType(t *testing.T) {
	conn := startGreeter(t, map[string]string{
		"bob.json": greeterStub("bob", "hi bob"),
	})

	got, err := callHello(t, conn, "bob")
	if err != nil {
		t.Fatalf("Hello(user_id=bob) error: %v", err)
	}
	if got != "hi bob" {
		t.Errorf("reply = %q, want %q", got, "hi bob")
	}
}

func TestGRPC_BodyMatchAcceptsCamelCaseFieldNames(t *testing.T) {
	conn := startGreeter(t, map[string]string{
		"bob.json": greeterStubWithField("userId", "bob", "hi camel bob"),
	})

	got, err := callHello(t, conn, "bob")
	if err != nil {
		t.Fatalf("Hello(user_id=bob) error: %v", err)
	}
	if got != "hi camel bob" {
		t.Errorf("reply = %q, want %q", got, "hi camel bob")
	}
}

func TestGRPC_BodyMatchOnLaterStub(t *testing.T) {
	// Stub files load in name order, so alice.json is tried before bob.json.
	conn := startGreeter(t, map[string]string{
		"alice.json": greeterStub("alice", "hi alice"),
		"bob.json":   greeterStub("bob", "hi bob"),
	})

	for _, user := range []string{"alice", "bob"} {
		t.Run(user, func(t *testing.T) {
			got, err := callHello(t, conn, user)
			if err != nil {
				t.Fatalf("Hello(user_id=%s) error: %v", user, err)
			}
			if want := "hi " + user; got != want {
				t.Errorf("reply = %q, want %q", got, want)
			}
		})
	}
}

func TestGRPC_UnmatchedCallsReturnStatusErrors(t *testing.T) {
	conn := startGreeter(t, map[string]string{
		"alice.json": greeterStub("alice", "hi alice"),
	})

	tests := []struct {
		name     string
		method   string
		userID   string
		wantCode codes.Code
	}{
		{"no stub for method", "/test.Greeter/Goodbye", "alice", codes.Unimplemented},
		{"stubs exist but none match", "/test.Greeter/Hello", "carol", codes.NotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := callGreeter(t, conn, tt.method, tt.userID)
			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("status code = %v, want %v (err: %v)", got, tt.wantCode, err)
			}
		})
	}
}
