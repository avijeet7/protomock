package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/httpserver"
	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/models"
	"github.com/avijeet7/protomock/internal/web"
)

func main() {
	// Check for grpcurl
	if _, err := exec.LookPath("grpcurl"); err != nil {
		log.Println("⚠️ grpcurl not found. gRPC request maker will not be available in the UI.")
	}

	var wg sync.WaitGroup

	var allHttpRoutes []models.Route
	var grpcRoutes []models.Route

	// Load HTTP routes
	httpPath := "mocks/http"
	if exists(httpPath) {
		protoHttpRoutes, err := loader.LoadProtoMocks(httpPath)
		if err != nil {
			log.Printf("❌ Failed to load proto-associated HTTP mocks: %v", err)
		} else {
			allHttpRoutes = append(allHttpRoutes, protoHttpRoutes...)
		}

		jsonHttpRoutes, err := loader.LoadJSONMocks(httpPath)
		if err != nil {
			log.Printf("❌ Failed to load standalone JSON HTTP mocks: %v", err)
		} else {
			allHttpRoutes = append(allHttpRoutes, jsonHttpRoutes...)
		}
	}

	// Load gRPC routes
	grpcPath := "mocks/grpc"
	if exists(grpcPath) {
		var err error
		grpcRoutes, err = loader.LoadProtoMocks(grpcPath)
		if err != nil {
			log.Printf("❌ Failed to load gRPC mocks: %v", err)
		}
	}

	// Start HTTP server
	if len(allHttpRoutes) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wd, _ := os.Getwd()
			uiHandler := web.NewUIHandler(allHttpRoutes, grpcRoutes, filepath.Join(wd, "internal", "web"))
			httpserver.StartHTTPServer(allHttpRoutes, uiHandler)
		}()
	} else {
		log.Println("ℹ️ No HTTP routes loaded, skipping HTTP server")
	}

	// Start gRPC server
	if len(grpcRoutes) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			grpcserver.StartGRPCServer(grpcRoutes)
		}()
	} else {
		log.Println("ℹ️ No gRPC routes loaded, skipping gRPC server")
	}

	log.Println("🌐 ProtoMock UI available at http://localhost:8085/protomock-ui")

	// Wait for servers to finish
	wg.Wait()
	log.Println("✅ ProtoMock shutdown cleanly")
}

func exists(path string) bool {
	_, err := os.Stat(filepath.Clean(path))
	return err == nil
}
