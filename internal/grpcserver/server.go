package grpcserver

import (
	"fmt"
	"log"
	"net"

	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// StartGRPCServer serves gRPC mocks on addr (e.g. ":8086").
// It blocks until the server stops and returns why, e.g. the port is in use.
func StartGRPCServer(addr string, routes []models.Route) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("gRPC server: %w", err)
	}

	log.Printf("🚀 gRPC server started on %s", listener.Addr())
	return NewGRPCServer(routes).Serve(listener)
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
