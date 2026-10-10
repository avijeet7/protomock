package grpcserver

import (
	"fmt"
	"log"
	"net"

	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/grpc"
)

// Server serves gRPC mocks. Its routes (and reflection) can be replaced while it runs (hot reload).
type Server struct {
	grpc       *grpc.Server
	mocks      *mockService
	reflection *reflectionSource
}

// NewServer returns a server that answers every method from the given stubs.
func NewServer(routes []models.Route) *Server {
	mocks := &mockService{}
	server := grpc.NewServer(grpc.UnknownServiceHandler(mocks.ServeGRPC))
	s := &Server{grpc: server, mocks: mocks, reflection: registerReflection(server)}
	s.Update(routes)
	return s
}

// Update atomically replaces the served routes; in-flight calls finish on the old ones.
func (s *Server) Update(routes []models.Route) {
	routeMap := groupRoutesByMethod(routes)
	s.mocks.routes.Store(&routeMap)
	s.reflection.update(routes)
	for fullMethod := range routeMap {
		log.Printf("[gRPC] Registered route: %s", fullMethod)
	}
}

// Serve accepts connections on lis until the server stops.
func (s *Server) Serve(lis net.Listener) error { return s.grpc.Serve(lis) }

// Stop closes all connections and listeners.
func (s *Server) Stop() { s.grpc.Stop() }

// ListenAndServe serves on addr (e.g. ":8086"). It blocks until the server stops
// and returns why, e.g. the port is in use.
func (s *Server) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("gRPC server: %w", err)
	}
	log.Printf("🚀 gRPC server started on %s", lis.Addr())
	return s.Serve(lis)
}
