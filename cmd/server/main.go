// Command server runs ProtoMock, a dynamic HTTP and gRPC mock server.
package main

import (
	"errors"
	"flag"
	"log"
	"os"

	"github.com/avijeet7/protomock/internal/app"
	"github.com/avijeet7/protomock/internal/config"
)

func main() {
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
