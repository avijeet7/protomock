package grpcserver

import (
	"log"
	"net"

	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func StartGRPCServer(routes []models.Route) {
	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("❌ Failed to listen on gRPC port: %v", err)
	}

	routeMap := normalizeRoutes(routes)

	server := grpc.NewServer(
		grpc.UnknownServiceHandler((&mockService{routes: routeMap}).ServeGRPC),
	)

	reflection.Register(server)

	for fullMethod := range routeMap {
		log.Printf("[gRPC] Registered route: %s", fullMethod)
	}

	log.Println("🚀 gRPC server started on :9090")
	if err := server.Serve(listener); err != nil {
		log.Fatalf("❌ Failed to serve gRPC server: %v", err)
	}
}
