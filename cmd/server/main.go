package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/httpserver"
	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/models"
	"github.com/avijeet7/protomock/internal/ui"
)

func main() {
	var wg sync.WaitGroup

	var allHttpRoutes []models.Route
	var grpcRoutes []models.Route

	// Start HTTP server only if mocks/http exists
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

		if len(allHttpRoutes) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				httpserver.StartHTTPServer(allHttpRoutes)
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
		var err error
		grpcRoutes, err = loader.LoadProtoMocks(grpcPath) // Assign to the higher-scoped grpcRoutes
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

	// Register UI handler
	http.HandleFunc("/protomock-ui", ui.GenerateUI(allHttpRoutes, grpcRoutes))
	log.Println("🌐 ProtoMock UI available at http://localhost:8085/protomock-ui")

	wg.Wait()
	log.Println("✅ ProtoMock shutdown cleanly")
}

func exists(path string) bool {
	_, err := os.Stat(filepath.Clean(path))
	return err == nil
}
