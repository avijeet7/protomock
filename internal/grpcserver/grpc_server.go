package grpcserver

import (
	"log"
	"net"
	"strings"

	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// mockService implements a generic gRPC server handler.
type mockService struct {
	routes map[string]models.Route
}

// ServeGRPC dynamically serves responses for any gRPC method registered via UnknownServiceHandler.
func (m *mockService) ServeGRPC(srv interface{}, serverStream grpc.ServerStream) error {
	fullMethod, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		log.Println("Unable to get method from stream")
		return nil
	}

	route, found := m.routes[fullMethod]
	if !found {
		log.Printf("❌ No gRPC mock route found for %s", fullMethod)
		return nil
	}

	if err := serverStream.SendMsg(route.Message); err != nil {
		log.Printf("⚠️ Failed to send mock gRPC response: %v", err)
	} else {
		log.Printf("✅ Served gRPC mock for %s", fullMethod)
	}
	return nil
}

// StartGRPCServer starts the gRPC server with dynamic handler and reflection.
func StartGRPCServer(routes []models.Route) {
	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port: %v", err)
	}

	// Normalize routes to fullMethod form: /package.Service/Method
	routeMap := make(map[string]models.Route)
	for _, r := range routes {
		key := r.URL
		if !strings.HasPrefix(key, "/") {
			key = "/" + key
		}
		routeMap[key] = r
	}

	server := grpc.NewServer(
		grpc.UnknownServiceHandler((&mockService{routes: routeMap}).ServeGRPC),
	)

	// Allow tools like grpcurl to query service metadata (even if we don’t register real service descriptors)
	reflection.Register(server)

	log.Println("🚀 gRPC server started on :9090")
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
