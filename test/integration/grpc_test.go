//go:build integration

// Package integration runs the real loader and servers end to end against mocks on disk.
package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/loader"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
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

	root := t.TempDir()
	files := map[string]string{"grpc/greeter/greeter.proto": greeterProto}
	for name, content := range stubs {
		files["grpc/greeter/stubs/"+name] = content
	}
	writeFiles(t, root, files)

	set, err := loader.LoadAll(root)
	if err != nil || len(set.GRPC) != len(stubs) {
		t.Fatalf("loaded %d routes (err=%v, problems=%v), want %d", len(set.GRPC), err, set.Problems, len(stubs))
	}
	routes := set.GRPC

	lis, err := net.Listen("tcp", "127.0.0.1:0") // port 0 = any free port
	if err != nil {
		t.Fatal(err)
	}
	server := grpcserver.NewServer(routes)
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

	fd := compileProto(t, "greeter.proto", greeterProto)
	reqDesc, replyDesc := fd.Messages().ByName("HelloRequest"), fd.Messages().ByName("HelloReply")

	req := dynamicpb.NewMessage(reqDesc)
	req.Set(reqDesc.Fields().ByName("user_id"), protoreflect.ValueOfString(userID))
	reply := dynamicpb.NewMessage(replyDesc)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Invoke(ctx, method, req, reply); err != nil {
		return "", err
	}
	return reply.Get(replyDesc.Fields().ByName("message")).String(), nil
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

func TestGRPC_BodyMatchOnZeroValueField(t *testing.T) {
	// proto3 doesn't send zero values on the wire; stubs should still be able to match them.
	conn := startGreeter(t, map[string]string{
		"empty.json": greeterStub("", "hi nobody"),
	})

	got, err := callHello(t, conn, "")
	if err != nil {
		t.Fatalf("Hello(user_id=\"\") error: %v", err)
	}
	if got != "hi nobody" {
		t.Errorf("reply = %q, want %q", got, "hi nobody")
	}
}

func TestGRPC_ResponseMetadataAndDelay(t *testing.T) {
	conn := startGreeter(t, map[string]string{
		"slow.json": `{
  "request": {"url": "/test.Greeter/Hello"},
  "response": {"headers": {"X-Trace-Id": "t-1"}, "delay_ms": 100, "body": {"message": "slow"}}
}`,
	})

	fd := compileProto(t, "greeter.proto", greeterProto)
	req := dynamicpb.NewMessage(fd.Messages().ByName("HelloRequest"))
	reply := dynamicpb.NewMessage(fd.Messages().ByName("HelloReply"))

	t.Run("metadata and delay", func(t *testing.T) {
		var header metadata.MD
		start := time.Now()
		if err := conn.Invoke(context.Background(), "/test.Greeter/Hello", req, reply, grpc.Header(&header)); err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
			t.Errorf("replied after %v, want at least 100ms", elapsed)
		}
		if got := header.Get("x-trace-id"); len(got) != 1 || got[0] != "t-1" {
			t.Errorf("x-trace-id metadata = %v, want [t-1]", got)
		}
	})

	t.Run("client deadline shorter than delay", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := conn.Invoke(ctx, "/test.Greeter/Hello", req, reply)
		if status.Code(err) != codes.DeadlineExceeded {
			t.Errorf("code = %v, want DeadlineExceeded", status.Code(err))
		}
	})
}
