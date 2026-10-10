package grpcserver

import (
	"log"
	"net"

	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func StartGRPCServer(routes []models.Route) {
	listener, err := net.Listen("tcp", ":8086")
	if err != nil {
		log.Fatalf("❌ Failed to listen on gRPC port: %v", err)
	}

	server := NewGRPCServer(routes)

	log.Println("🚀 gRPC server started on :8086")
	if err := server.Serve(listener); err != nil {
		log.Fatalf("❌ Failed to serve gRPC server: %v", err)
	}
}

// NewGRPCServer builds a gRPC server that answers every method from the given stubs.
func NewGRPCServer(routes []models.Route) *grpc.Server {
	// 🔁 Group routes by gRPC full method path
	routeMap := groupRoutesByMethod(routes)

	server := grpc.NewServer(
		grpc.UnknownServiceHandler((&mockService{routes: routeMap}).ServeGRPC),
	)

	reflection.Register(server)

	for fullMethod := range routeMap {
		log.Printf("[gRPC] Registered route: %s", fullMethod)
	}

	return server
}
