package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/avijeet7/protomock/internal/loader"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

const tryTimeout = 30 * time.Second

// POST /try/http {method, path, headers, body, kind?, mock?, message?}
// Sends a request through the HTTP mock router (in-process) and returns the response.
// If the response is protobuf and mock+message are given, the body is also decoded to JSON.
func (a *api) tryHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method  string            `json:"method"`
		Path    string            `json:"path"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
		Kind    loader.Kind       `json:"kind"`
		Mock    string            `json:"mock"`
		Message string            `json:"message"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	if !strings.HasPrefix(req.Path, "/") {
		writeErrorf(w, http.StatusBadRequest, "path must start with /")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tryTimeout)
	defer cancel()
	mockReq := httptest.NewRequest(strings.ToUpper(req.Method), req.Path, strings.NewReader(req.Body)).WithContext(ctx)
	for k, v := range req.Headers {
		mockReq.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	start := time.Now()
	a.backend.ServeMock(rec, mockReq)
	elapsed := time.Since(start)

	resp := rec.Result()
	headers := map[string]string{}
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}
	body := rec.Body.Bytes()
	out := map[string]any{
		"status":      resp.StatusCode,
		"headers":     headers,
		"duration_ms": elapsed.Milliseconds(),
	}
	if utf8.Valid(body) && !strings.Contains(resp.Header.Get("Content-Type"), "protobuf") {
		out["body"] = string(body)
	} else {
		out["body_base64"] = base64.StdEncoding.EncodeToString(body)
		out["body_size"] = len(body)
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "protobuf") && req.Message != "" {
		if decoded, err := a.decodeProto(req.Kind, req.Mock, req.Message, body); err == nil {
			out["body_json"] = decoded
		} else {
			out["decode_error"] = err.Error()
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) decodeProto(kind loader.Kind, mockName, message string, data []byte) (json.RawMessage, error) {
	mock := a.findMock(kind, mockName)
	if mock == nil {
		return nil, fmt.Errorf("mock %s/%s not found", kind, mockName)
	}
	md := mock.Protos.Message(message)
	if md == nil {
		return nil, fmt.Errorf("message type %q not found", message)
	}
	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return protoJSON(msg)
}

// POST /try/grpc {method, metadata, body}
// Calls a unary gRPC method on the running mock server and returns the reply.
func (a *api) tryGRPC(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method   string            `json:"method"` // "/package.Service/Method"
		Metadata map[string]string `json:"metadata"`
		Body     json.RawMessage   `json:"body"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !strings.HasPrefix(req.Method, "/") {
		req.Method = "/" + req.Method
	}
	service, method, _ := strings.Cut(strings.TrimPrefix(req.Method, "/"), "/")
	md := a.findMethod(service, method)
	if md == nil {
		writeErrorf(w, http.StatusNotFound, "method %s not found in any gRPC mock's .proto files", req.Method)
		return
	}
	if md.IsStreamingClient() || md.IsStreamingServer() {
		writeErrorf(w, http.StatusBadRequest, "Try it supports unary methods only")
		return
	}

	in := dynamicpb.NewMessage(md.Input())
	if len(req.Body) > 0 && string(req.Body) != "null" {
		if err := protojson.Unmarshal(req.Body, in); err != nil {
			writeErrorf(w, http.StatusBadRequest, "body doesn't match %s: %v", md.Input().FullName(), err)
			return
		}
	}
	reply := dynamicpb.NewMessage(md.Output())

	conn, err := grpc.NewClient(a.backend.GRPCTarget(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		writeErrorf(w, http.StatusBadGateway, "connecting to the gRPC server: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(r.Context(), tryTimeout)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.New(req.Metadata))

	var header metadata.MD
	start := time.Now()
	callErr := conn.Invoke(ctx, req.Method, in, reply, grpc.Header(&header))
	elapsed := time.Since(start)

	st := status.Convert(callErr)
	out := map[string]any{
		"code":        st.Code().String(),
		"message":     st.Message(),
		"headers":     flattenMD(header),
		"duration_ms": elapsed.Milliseconds(),
	}
	if callErr == nil {
		if body, err := protoJSON(reply); err == nil {
			out["body"] = body
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) findMethod(service, method string) protoreflect.MethodDescriptor {
	for _, m := range a.backend.Set().Mocks {
		if m.Kind != loader.KindGRPC {
			continue
		}
		if md := m.Protos.Method(service, method); md != nil {
			return md
		}
	}
	return nil
}

// protoJSON renders a message as indented JSON with .proto field names and all fields shown.
func protoJSON(msg proto.Message) (json.RawMessage, error) {
	data, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", "  ") // stable formatting (protojson's spacing is randomized)
}

func flattenMD(md metadata.MD) map[string]string {
	out := map[string]string{}
	for k, v := range md {
		out[k] = strings.Join(v, ", ")
	}
	return out
}
