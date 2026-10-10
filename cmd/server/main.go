package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/avijeet7/protomock/internal/config"
	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/httpserver"
	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/models"
	"github.com/avijeet7/protomock/internal/web"
)

func main() {
	cfg, err := config.Load(os.Args[1:], os.Getenv, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		os.Exit(2)
	}

	// Check for grpcurl
	if _, err := exec.LookPath("grpcurl"); err != nil {
		log.Println("⚠️ grpcurl not found. gRPC request maker will not be available in the UI.")
	}

	var allHttpRoutes []models.Route
	var grpcRoutes []models.Route

	// Load HTTP routes
	httpPath := filepath.Join(cfg.MocksDir, "http")
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
	grpcPath := filepath.Join(cfg.MocksDir, "grpc")
	if exists(grpcPath) {
		grpcRoutes, err = loader.LoadProtoMocks(grpcPath)
		if err != nil {
			log.Printf("❌ Failed to load gRPC mocks: %v", err)
		}
	}

	webDir, err := filepath.Abs(cfg.WebDir)
	if err != nil {
		log.Fatalf("❌ Invalid web dir %q: %v", cfg.WebDir, err)
	}

	// Each server reports here if it stops; any failure (e.g. port in use) exits the process.
	errCh := make(chan error, 2)

	// Start HTTP server (always: it also serves the UI, even with no HTTP mocks)
	if len(allHttpRoutes) == 0 {
		log.Println("ℹ️ No HTTP routes loaded; HTTP server will only serve the UI")
	}
	uiHandler := web.NewUIHandler(allHttpRoutes, grpcRoutes, webDir)
	go func() { errCh <- httpserver.StartHTTPServer(cfg.HTTPAddr, allHttpRoutes, uiHandler) }()

	// Start gRPC server
	if len(grpcRoutes) > 0 {
		go func() { errCh <- grpcserver.StartGRPCServer(cfg.GRPCAddr, grpcRoutes) }()
	} else {
		log.Println("ℹ️ No gRPC routes loaded, skipping gRPC server")
	}

	log.Printf("🌐 ProtoMock UI available at %s/protomock-ui", uiURL(cfg.HTTPAddr))

	log.Fatalf("❌ %v", <-errCh)
}

// uiURL turns a listen address like ":8085" into a browsable base URL.
func uiURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	return "http://" + addr
}

func exists(path string) bool {
	_, err := os.Stat(filepath.Clean(path))
	return err == nil
}
