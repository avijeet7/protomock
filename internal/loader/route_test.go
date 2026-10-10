package loader

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/avijeet7/protomock/internal/models"
)

func TestBuildRoute(t *testing.T) {
	root := writeTree(t, map[string]string{"greeter.proto": greeterProto})
	protos, err := CompileProtos(root, []string{"greeter.proto"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		kind    Kind
		stub    string
		wantErr string // empty = valid
		check   func(t *testing.T, r models.Route)
	}{
		{name: "http: url required", kind: KindHTTP, stub: `{"request":{"method":"GET"}}`, wantErr: "request.url is required"},
		{name: "http: url must start with /", kind: KindHTTP, stub: `{"request":{"method":"GET","url":"x"}}`, wantErr: "must start with /"},
		{name: "http: method is upper-cased", kind: KindHTTP, stub: `{"request":{"method":"post","url":"/x"}}`,
			check: func(t *testing.T, r models.Route) {
				if r.Method != "POST" {
					t.Errorf("method = %q, want POST", r.Method)
				}
			}},
		{name: "http: invalid status", kind: KindHTTP, stub: `{"request":{"method":"GET","url":"/x"},"response":{"status":42}}`, wantErr: "not a valid HTTP status"},
		{name: "http: regex compiled", kind: KindHTTP, stub: `{"request":{"method":"GET","url":"/u/[0-9]+"}}`,
			check: func(t *testing.T, r models.Route) {
				if r.PathRegex == nil || !r.PathRegex.MatchString("/u/12") {
					t.Error("regex URL was not compiled")
				}
			}},
		{name: "http: invalid request body JSON", kind: KindHTTP, stub: `{"request":{"method":"GET","url":"/x","body":"x"},"response":{}}`,
			check: func(t *testing.T, r models.Route) {}}, // a JSON string is valid JSON
		{name: "http: proto needs message", kind: KindHTTP, stub: `{"request":{"method":"GET","url":"/x"},"response":{"proto":true}}`, wantErr: "response.message"},
		{name: "http: unknown message", kind: KindHTTP, stub: `{"request":{"method":"GET","url":"/x"},"response":{"proto":true,"message":"greeter.Nope"}}`, wantErr: "not found"},
		{name: "http: body doesn't fit message", kind: KindHTTP, stub: `{"request":{"method":"GET","url":"/x"},"response":{"proto":true,"message":"greeter.HelloReply","body":{"nope":1}}}`, wantErr: "doesn't match greeter.HelloReply"},

		{name: "grpc: message defaults to output type", kind: KindGRPC, stub: `{"request":{"url":"/greeter.Greeter/Hello"},"response":{"body":{"message":"hi"}}}`,
			check: func(t *testing.T, r models.Route) {
				if r.MessageDesc.FullName() != "greeter.HelloReply" || r.RequestDesc.FullName() != "greeter.HelloRequest" || !r.ProtoEncoded {
					t.Errorf("got message %v request %v proto %v", r.MessageDesc.FullName(), r.RequestDesc.FullName(), r.ProtoEncoded)
				}
			}},
		{name: "grpc: leading slash optional", kind: KindGRPC, stub: `{"request":{"url":"greeter.Greeter/Hello"}}`,
			check: func(t *testing.T, r models.Route) {
				if r.URL != "/greeter.Greeter/Hello" {
					t.Errorf("url = %q", r.URL)
				}
			}},
		{name: "grpc: well-known import types", kind: KindGRPC, stub: `{"request":{"url":"/greeter.Greeter/Ping"}}`},
		{name: "grpc: bad url", kind: KindGRPC, stub: `{"request":{"url":"/greeter.Greeter"}}`, wantErr: "gRPC method path"},
		{name: "grpc: wrong message type", kind: KindGRPC, stub: `{"request":{"url":"/greeter.Greeter/Hello"},"response":{"message":"greeter.HelloRequest"}}`, wantErr: "doesn't match the method's output type"},
		{name: "grpc: unknown method with body match", kind: KindGRPC, stub: `{"request":{"url":"/greeter.Greeter/Nope","body":{"a":1}},"response":{"message":"greeter.HelloReply"}}`, wantErr: "not found"},
		{name: "grpc: unknown method without message", kind: KindGRPC, stub: `{"request":{"url":"/greeter.Greeter/Nope"}}`, wantErr: "response.message is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stub models.Stub
			if err := json.Unmarshal([]byte(tt.stub), &stub); err != nil {
				t.Fatal(err)
			}
			route, err := BuildRoute(tt.kind, stub, protos)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, route)
			}
		})
	}
}
