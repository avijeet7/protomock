package main

import (
	"log"
	"sync"

	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/httpserver"
	"github.com/avijeet7/protomock/internal/loader"
)

func main() {
	httpRoutes, err := loader.LoadMocks("./mocks/http")
	if err != nil {
		log.Fatalf("Failed to load HTTP mocks: %v", err)
	}

	grpcRoutes, err := loader.LoadMocks("./mocks/grpc")
	if err != nil {
		log.Fatalf("Failed to load gRPC mocks: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		httpserver.StartHTTPServer(httpRoutes)
	}()

	go func() {
		defer wg.Done()
		grpcserver.StartGRPCServer(grpcRoutes)
	}()

	log.Println("Both HTTP and gRPC servers started")
	wg.Wait()
}
