package grpcserver

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/avijeet7/protomock/internal/match"
	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"
)

type mockService struct {
	routes atomic.Pointer[map[string][]models.Route] // gRPC full method -> candidate stubs
}

func (m *mockService) ServeGRPC(_ interface{}, serverStream grpc.ServerStream) error {
	fullMethod, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		log.Println("❌ Unable to determine method from gRPC stream")
		return status.Error(codes.Internal, "protomock: unable to determine method from gRPC stream")
	}

	routes := *m.routes.Load()
	candidates, found := routes[fullMethod]
	if !found {
		msg := unimplementedMessage(fullMethod, routes)
		log.Printf("[gRPC] ❌ %s", msg)
		return status.Error(codes.Unimplemented, msg)
	}

	// The request can only be received once, so read it before trying candidates.
	reqMsg, err := receiveRequest(serverStream, candidates)
	if err != nil {
		log.Printf("❌ Failed to receive gRPC request body for %s: %v", fullMethod, err)
		return status.Errorf(codes.Internal, "protomock: failed to read request for %s: %v", fullMethod, err)
	}
	md, _ := metadata.FromIncomingContext(serverStream.Context())

	var misses []nearMiss
	for _, route := range candidates {
		if reasons := mismatches(route, md, reqMsg); len(reasons) > 0 {
			misses = append(misses, nearMiss{stub: route.Name(), reasons: reasons})
			continue
		}

		if !route.WaitDelay(serverStream.Context()) {
			return status.FromContextError(serverStream.Context().Err()).Err()
		}
		if len(route.Headers) > 0 {
			if err := serverStream.SetHeader(metadata.New(route.Headers)); err != nil {
				log.Printf("⚠️ Failed to set gRPC response metadata for %s: %v", fullMethod, err)
			}
		}
		if err := serverStream.SendMsg(route.Message); err != nil {
			log.Printf("⚠️ Failed to send gRPC response for %s: %v", fullMethod, err)
			return err
		}
		log.Printf("[gRPC] ✅ Serving %s for %s", route.Name(), fullMethod)
		return nil
	}

	msg := notFoundMessage(fullMethod, misses)
	log.Printf("[gRPC] ❌ %s", msg)
	return status.Error(codes.NotFound, msg)
}

// nearMiss is a stub for the called method and the rules the request failed.
type nearMiss struct {
	stub    string
	reasons []string
}

// mismatches lists the rules of route that the request fails (empty = it matches).
func mismatches(route models.Route, md metadata.MD, reqMsg *dynamicpb.Message) []string {
	reasons := metadataMismatches(route.HeaderMatch, md)
	if len(route.BodyMatch) == 0 {
		return reasons
	}
	switch {
	case reqMsg == nil:
		reasons = append(reasons, "body: the method isn't in the mock's .proto files, so the body can't be matched")
	case !bodyMatches(route.BodyMatch, reqMsg):
		actual, _ := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(reqMsg)
		reasons = append(reasons, match.ExplainJSON(route.BodyMatch, actual)...)
	}
	return reasons
}

// notFoundMessage explains the closest stub, e.g. `no stub matched /pkg.S/M (2 stubs). Closest
// grpc/s/stubs/a.json: metadata x-tenant: expected "acme", got "other"`.
func notFoundMessage(fullMethod string, misses []nearMiss) string {
	sort.SliceStable(misses, func(i, j int) bool { return len(misses[i].reasons) < len(misses[j].reasons) })
	noun := "stubs"
	if len(misses) == 1 {
		noun = "stub"
	}
	msg := fmt.Sprintf("protomock: no stub matched %s (%d %s)", fullMethod, len(misses), noun)
	if len(misses) > 0 {
		msg += fmt.Sprintf(". Closest %s: %s", misses[0].stub, strings.Join(misses[0].reasons, "; "))
	}
	return msg
}

// unimplementedMessage lists the mocked methods, so a typo in the service or method name is obvious.
func unimplementedMessage(fullMethod string, routes map[string][]models.Route) string {
	if len(routes) == 0 {
		return fmt.Sprintf("protomock: no stubs for %s (no gRPC mocks are loaded)", fullMethod)
	}
	methods := make([]string, 0, len(routes))
	for m := range routes {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	more := ""
	if len(methods) > 10 {
		more = fmt.Sprintf(" and %d more", len(methods)-10)
		methods = methods[:10]
	}
	return fmt.Sprintf("protomock: no stubs for %s. Mocked methods: %s%s", fullMethod, strings.Join(methods, ", "), more)
}

// receiveRequest reads the single request message, decoded with the method's
// input type. All candidates share a method, so any known RequestDesc works.
// It returns a nil message if the input type is unknown.
func receiveRequest(stream grpc.ServerStream, candidates []models.Route) (*dynamicpb.Message, error) {
	for _, route := range candidates {
		if route.RequestDesc != nil {
			reqMsg := dynamicpb.NewMessage(route.RequestDesc)
			if err := stream.RecvMsg(reqMsg); err != nil {
				return nil, err
			}
			return reqMsg, nil
		}
	}
	return nil, nil
}

// headersMatch reports whether every expected header is present in the
// request metadata with the expected (first) value.
func headersMatch(expected map[string]string, md metadata.MD) bool {
	return len(metadataMismatches(expected, md)) == 0
}

// metadataMismatches explains which expected metadata entries are missing or different.
func metadataMismatches(expected map[string]string, md metadata.MD) []string {
	keys := make([]string, 0, len(expected))
	for k := range expected {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var reasons []string
	for _, key := range keys {
		want, values := expected[key], md.Get(key)
		switch {
		case len(values) == 0:
			reasons = append(reasons, fmt.Sprintf("metadata %s: missing (expected %q)", strings.ToLower(key), want))
		case values[0] != want:
			reasons = append(reasons, fmt.Sprintf("metadata %s: expected %q, got %q", strings.ToLower(key), want, values[0]))
		}
	}
	return reasons
}

// bodyMatches checks the stub's body against the request rendered as JSON.
// Stubs may use either the .proto field names (user_id) or proto3 JSON
// camelCase names (userId), so both renderings are tried. Unset fields are
// rendered with their zero values so stubs can match on e.g. {"count": 0}.
func bodyMatches(expected []byte, reqMsg *dynamicpb.Message) bool {
	for _, opts := range []protojson.MarshalOptions{
		{UseProtoNames: true, EmitUnpopulated: true},
		{EmitUnpopulated: true},
	} {
		if actual, err := opts.Marshal(reqMsg); err == nil && match.PartialJSON(expected, actual) {
			return true
		}
	}
	return false
}
