package grpcserver

import (
	"log"

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
	routes map[string][]models.Route
}

func (m *mockService) ServeGRPC(_ interface{}, serverStream grpc.ServerStream) error {
	fullMethod, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		log.Println("❌ Unable to determine method from gRPC stream")
		return status.Error(codes.Internal, "protomock: unable to determine method from gRPC stream")
	}

	candidates, found := m.routes[fullMethod]
	if !found {
		log.Printf("❌ No gRPC mock candidates for %s", fullMethod)
		return status.Errorf(codes.Unimplemented, "protomock: no stubs configured for %s", fullMethod)
	}

	// The request can only be received once, so read it before trying candidates.
	reqMsg, err := receiveRequest(serverStream, candidates)
	if err != nil {
		log.Printf("❌ Failed to receive gRPC request body for %s: %v", fullMethod, err)
		return status.Errorf(codes.Internal, "protomock: failed to read request for %s: %v", fullMethod, err)
	}
	md, _ := metadata.FromIncomingContext(serverStream.Context())

	for _, route := range candidates {
		if !headersMatch(route.HeaderMatch, md) {
			continue
		}

		// ✅ Body matching (only if specified)
		if len(route.BodyMatch) > 0 {
			if reqMsg == nil {
				log.Printf("❌ Cannot match body for %s: method not found in the stub's proto service definitions", fullMethod)
				continue
			}
			if !bodyMatches(route.BodyMatch, reqMsg) {
				log.Printf("❌ Body mismatch for %s (%s)", fullMethod, route.FilePath)
				continue
			}
		}

		// ✅ Send response
		if !route.ProtoEncoded {
			log.Printf("⚠️ Stub for %s is not marked as proto; skipping response", fullMethod)
			continue
		}

		if err := serverStream.SendMsg(route.Message); err != nil {
			log.Printf("⚠️ Failed to send gRPC response for %s: %v", fullMethod, err)
			return err
		}
		log.Printf("✅ Served gRPC mock for %s", fullMethod)
		return nil
	}

	log.Printf("❌ No gRPC stub matched for %s", fullMethod)
	return status.Errorf(codes.NotFound, "protomock: none of the %d stubs for %s matched the request headers/body", len(candidates), fullMethod)
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
	for key, want := range expected {
		values := md.Get(key)
		if len(values) == 0 || values[0] != want {
			return false
		}
	}
	return true
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
