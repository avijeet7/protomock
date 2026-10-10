// Package app wires ProtoMock together: it loads the mocks folder and runs the
// HTTP (mocks + UI) and gRPC servers.
package app

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/avijeet7/protomock/internal/config"
	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/httpserver"
	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/web"
)

// Run loads the mocks and serves them until a server fails (e.g. its port is in use).
func Run(cfg config.Config) error {
	set, err := loader.LoadAll(cfg.MocksDir)
	if err != nil {
		return fmt.Errorf("loading mocks from %s: %w", cfg.MocksDir, err)
	}
	logSet(set)

	webDir, err := filepath.Abs(cfg.WebDir)
	if err != nil {
		return fmt.Errorf("invalid web dir %q: %w", cfg.WebDir, err)
	}

	errCh := make(chan error, 2)

	// The HTTP server always runs: it also serves the UI, even with no HTTP mocks.
	uiHandler := web.NewUIHandler(set.HTTP, set.GRPC, webDir)
	go func() { errCh <- httpserver.StartHTTPServer(cfg.HTTPAddr, set.HTTP, uiHandler) }()

	if len(set.GRPC) > 0 {
		go func() { errCh <- grpcserver.StartGRPCServer(cfg.GRPCAddr, set.GRPC) }()
	} else {
		log.Println("ℹ️ No gRPC routes loaded, skipping gRPC server")
	}

	log.Printf("🌐 ProtoMock UI available at %s/protomock-ui", baseURL(cfg.HTTPAddr))
	return <-errCh
}

func logSet(set *loader.Set) {
	log.Printf("📦 Loaded %d mocks: %d HTTP stubs, %d gRPC stubs", len(set.Mocks), len(set.HTTP), len(set.GRPC))
	if len(set.HTTP) == 0 {
		log.Println("ℹ️ No HTTP routes loaded; HTTP server will only serve the UI")
	}
	for _, p := range set.Problems {
		log.Printf("⚠️ Skipped %s: %s", p.Path, p.Error)
	}
}

// baseURL turns a listen address like ":8085" into a browsable base URL.
func baseURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	return "http://" + addr
}
