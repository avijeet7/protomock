package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/httpserver"
	"github.com/avijeet7/protomock/internal/loader"
)

func main() {
	var wg sync.WaitGroup

	// Start HTTP server only if mocks/http exists
	httpPath := "mocks/http"
	if exists(httpPath) {
		httpRoutes, err := loader.LoadMocks(httpPath)
		if err != nil {
			log.Printf("❌ Failed to load HTTP mocks: %v", err)
		} else if len(httpRoutes) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				httpserver.StartHTTPServer(httpRoutes)
			}()
		} else {
			log.Println("ℹ️ No HTTP routes loaded")
		}
	} else {
		log.Println("⏩ Skipping HTTP server: mocks/http not found")
	}

	// Start gRPC server only if mocks/grpc exists
	grpcPath := "mocks/grpc"
	if exists(grpcPath) {
		grpcRoutes, err := loader.LoadMocks(grpcPath)
		if err != nil {
			log.Printf("❌ Failed to load gRPC mocks: %v", err)
		} else if len(grpcRoutes) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				grpcserver.StartGRPCServer(grpcRoutes)
			}()
		} else {
			log.Println("ℹ️ No gRPC routes loaded")
		}
	} else {
		log.Println("⏩ Skipping gRPC server: mocks/grpc not found")
	}

	wg.Wait()
	log.Println("✅ ProtoMock shutdown cleanly")
}

func exists(path string) bool {
	_, err := os.Stat(filepath.Clean(path))
	return err == nil
}
