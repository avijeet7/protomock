// Command server runs ProtoMock, a dynamic HTTP and gRPC mock server.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/avijeet7/protomock/internal/app"
	"github.com/avijeet7/protomock/internal/config"
)

func main() {
	// `protomock healthcheck [flags]` checks a running server (exit 0 = healthy).
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		cfg, err := config.Load(os.Args[2:], os.Getenv, os.Stderr)
		if err != nil {
			os.Exit(2)
		}
		if err := app.HealthCheck(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "unhealthy:", err)
			os.Exit(1)
		}
		fmt.Println("ok")
		return
	}

	cfg, err := config.Load(os.Args[1:], os.Getenv, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		os.Exit(2)
	}

	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}
	log.Fatalf("❌ %v", a.Run())
}
