package loader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/avijeet7/protomock/internal/match"
	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// StubError is a validation error in a stub's content.
type StubError struct{ Msg string }

func (e *StubError) Error() string { return e.Msg }

func stubErrorf(format string, args ...any) error {
	return &StubError{Msg: fmt.Sprintf(format, args...)}
}

// BuildRoute validates a stub for the given kind and builds its route.
// protos are the mock's compiled .proto files (nil if it has none).
//
// Rules:
//   - request.url is required. HTTP URLs are a literal path or an anchored regex;
//     gRPC URLs are "/package.Service/Method".
//   - HTTP: request.method is required; response.status defaults to 200; with
//     response.proto the body is encoded as response.message.
//   - gRPC: responses are always protobuf; response.message defaults to the
//     method's output type; request.body matching needs the method in the protos.
func BuildRoute(kind Kind, stub models.Stub, protos *Protos) (models.Route, error) {
	req, resp := stub.Request, stub.Response

	if req.URL == "" {
		return models.Route{}, stubErrorf("request.url is required")
	}
	if len(req.Body) > 0 && !json.Valid(req.Body) {
		return models.Route{}, stubErrorf("request.body is not valid JSON")
	}
	if len(resp.Body) > 0 && !json.Valid(resp.Body) {
		return models.Route{}, stubErrorf("response.body is not valid JSON")
	}

	if resp.DelayMs < 0 {
		return models.Route{}, stubErrorf("response.delay_ms must not be negative")
	}

	route := models.Route{
		URL:         req.URL,
		Method:      strings.ToUpper(req.Method),
		HeaderMatch: req.Headers,
		BodyMatch:   req.Body,
		Status:      resp.Status,
		Headers:     resp.Headers,
		Delay:       time.Duration(resp.DelayMs) * time.Millisecond,
		Priority:    stub.Priority,
	}

	switch kind {
	case KindHTTP:
		return buildHTTPRoute(route, resp, protos)
	case KindGRPC:
		return buildGRPCRoute(route, resp, protos)
	default:
		return models.Route{}, stubErrorf("unknown mock kind %q", kind)
	}
}

func buildHTTPRoute(route models.Route, resp models.ResponseStub, protos *Protos) (models.Route, error) {
	if route.Method == "" {
		return models.Route{}, stubErrorf("request.method is required for HTTP stubs (e.g. GET, POST)")
	}
	if !strings.HasPrefix(route.URL, "/") {
		return models.Route{}, stubErrorf("request.url must start with /")
	}
	if match.IsRegexURL(route.URL) {
		re, err := match.CompileURL(route.URL)
		if err != nil {
			return models.Route{}, stubErrorf("request.url is not a valid regex: %v", err)
		}
		route.PathRegex = re
	}
	if route.Status == 0 {
		route.Status = http.StatusOK
	}
	if route.Status < 100 || route.Status > 999 {
		return models.Route{}, stubErrorf("response.status %d is not a valid HTTP status", route.Status)
	}

	if !resp.Proto {
		route.RawJSONBody = resp.Body
		return route, nil
	}

	if resp.Message == "" {
		return models.Route{}, stubErrorf("response.message (the protobuf message type) is required when response.proto is true")
	}
	md := protos.Message(resp.Message)
	if md == nil {
		return models.Route{}, stubErrorf("message type %q not found in the mock's .proto files", resp.Message)
	}
	msg, err := newMessage(md, resp.Body)
	if err != nil {
		return models.Route{}, stubErrorf("response.body doesn't match %s: %v", resp.Message, err)
	}
	route.ProtoEncoded = true
	route.MessageDesc = md
	route.Message = msg
	return route, nil
}

func buildGRPCRoute(route models.Route, resp models.ResponseStub, protos *Protos) (models.Route, error) {
	if !strings.HasPrefix(route.URL, "/") {
		route.URL = "/" + route.URL
	}
	service, method, ok := strings.Cut(strings.TrimPrefix(route.URL, "/"), "/")
	if !ok || service == "" || method == "" || strings.Contains(method, "/") {
		return models.Route{}, stubErrorf(`request.url must be a gRPC method path like "/package.Service/Method"`)
	}

	md := protos.Method(service, method)
	if md != nil {
		route.RequestDesc = md.Input()
	} else if len(route.BodyMatch) > 0 {
		return models.Route{}, stubErrorf("method %s/%s not found in the mock's .proto files (needed for request.body matching)", service, method)
	}

	msgName := resp.Message
	if msgName == "" {
		if md == nil {
			return models.Route{}, stubErrorf("response.message is required (method %s/%s not found in the mock's .proto files)", service, method)
		}
		msgName = string(md.Output().FullName())
	}
	msgDesc := protos.Message(msgName)
	if msgDesc == nil {
		return models.Route{}, stubErrorf("message type %q not found in the mock's .proto files", msgName)
	}
	if md != nil && msgDesc.FullName() != md.Output().FullName() {
		return models.Route{}, stubErrorf("response.message %s doesn't match the method's output type %s", msgName, md.Output().FullName())
	}
	msg, err := newMessage(msgDesc, resp.Body)
	if err != nil {
		return models.Route{}, stubErrorf("response.body doesn't match %s: %v", msgName, err)
	}

	route.Method = ""
	route.ProtoEncoded = true
	route.MessageDesc = msgDesc
	route.Message = msg
	return route, nil
}

// newMessage builds a dynamic message of type md from JSON (an empty body means an empty message).
func newMessage(md protoreflect.MessageDescriptor, body json.RawMessage) (*dynamicpb.Message, error) {
	msg := dynamicpb.NewMessage(md)
	if len(body) == 0 {
		return msg, nil
	}
	return msg, protojson.Unmarshal(body, msg)
}
