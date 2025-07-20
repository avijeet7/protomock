package grpcserver

import (
	"github.com/jhump/protoreflect/dynamic"
	"log"

	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type mockService struct {
	routes map[string]models.Route
}

func (m *mockService) ServeGRPC(_ interface{}, serverStream grpc.ServerStream) error {
	fullMethod, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		log.Println("❌ Unable to determine method from gRPC stream")
		return nil
	}

	route, found := m.routes[fullMethod]
	if !found {
		log.Printf("❌ No gRPC mock found for %s", fullMethod)
		return nil
	}

	// ✅ Header matching (if specified)
	if len(route.HeaderMatch) > 0 {
		md, ok := metadata.FromIncomingContext(serverStream.Context())
		if !ok {
			log.Printf("❌ Failed to extract metadata for %s", fullMethod)
			return nil
		}
		for key, expected := range route.HeaderMatch {
			values := md.Get(key)
			if len(values) == 0 || values[0] != expected {
				log.Printf("❌ Header %s mismatch for %s: expected=%s, actual=%v", key, fullMethod, expected, values)
				return nil
			}
		}
	}

	// ✅ Body matching (if specified)
	if len(route.BodyMatch) > 0 {
		reqMsg := dynamic.NewMessage(route.MessageDesc)
		if err := serverStream.RecvMsg(reqMsg); err != nil {
			log.Printf("❌ Failed to receive gRPC request body for %s: %v", fullMethod, err)
			return nil
		}

		actualBody, _ := reqMsg.MarshalJSON()
		if !partialJSONMatch(route.BodyMatch, actualBody) {
			log.Printf("❌ Body mismatch for %s", fullMethod)
			return nil
		}
	}

	// ✅ Send response
	if !route.ProtoEncoded {
		log.Printf("⚠️ Stub for %s is not marked as proto; skipping gRPC response", fullMethod)
		return nil
	}

	if err := serverStream.SendMsg(route.Message); err != nil {
		log.Printf("⚠️ Failed to send gRPC response for %s: %v", fullMethod, err)
	} else {
		log.Printf("✅ Served gRPC mock for %s", fullMethod)
	}
	return nil
}
