package grpcserver

import (
	"log"

	"github.com/avijeet7/protomock/internal/models"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type mockService struct {
	routes map[string][]models.Route
}

func (m *mockService) ServeGRPC(_ interface{}, serverStream grpc.ServerStream) error {
	fullMethod, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		log.Println("❌ Unable to determine method from gRPC stream")
		return nil
	}

	candidates, found := m.routes[fullMethod]
	if !found {
		log.Printf("❌ No gRPC mock candidates for %s", fullMethod)
		return nil
	}

	for _, route := range candidates {
		// ✅ Header matching (only if headers specified)
		if len(route.HeaderMatch) > 0 {
			md, ok := metadata.FromIncomingContext(serverStream.Context())
			if !ok {
				log.Printf("❌ Failed to extract metadata for %s", fullMethod)
				continue
			}
			matched := true
			for key, expected := range route.HeaderMatch {
				values := md.Get(key)
				if len(values) == 0 || values[0] != expected {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
		}

		// ✅ Body matching (only if specified)
		if len(route.BodyMatch) > 0 {
			reqMsg := dynamic.NewMessage(route.MessageDesc)
			if err := serverStream.RecvMsg(reqMsg); err != nil {
				log.Printf("❌ Failed to receive gRPC request body for %s: %v", fullMethod, err)
				continue
			}
			actualBody, _ := reqMsg.MarshalJSON()
			if !partialJSONMatch(route.BodyMatch, actualBody) {
				log.Printf("❌ Body mismatch for %s", fullMethod)
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
		} else {
			log.Printf("✅ Served gRPC mock for %s", fullMethod)
		}
		return nil
	}

	log.Printf("❌ No gRPC stub matched for %s", fullMethod)
	return nil
}
