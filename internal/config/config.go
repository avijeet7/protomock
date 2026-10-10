// Package config resolves ProtoMock settings from flags, environment variables and defaults.
// Precedence: command-line flag > environment variable > default.
package config

import (
	"flag"
	"fmt"
	"io"
	"time"
)

// Config holds the server settings.
type Config struct {
	HTTPAddr string // HTTP mocks + UI listen address
	GRPCAddr string // gRPC mocks listen address
	MocksDir string // root folder containing http/ and grpc/ mocks
	WebDir   string // optional: serve UI files from this folder instead of the embedded copy

	ReloadInterval time.Duration // how often to check the mocks folder for changes (0 = never)
}

// Load parses args (without the program name) using getenv for environment lookups;
// usage and parse errors are written to output. Passing these in, instead of using
// os.Args/os.Getenv/os.Stderr directly, keeps Load easy to test.
func Load(args []string, getenv func(string) string, output io.Writer) (Config, error) {
	envOr := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}

	var cfg Config
	fs := flag.NewFlagSet("protomock", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&cfg.HTTPAddr, "http-addr", envOr("PROTOMOCK_HTTP_ADDR", ":8085"), "HTTP mocks and UI listen address (env PROTOMOCK_HTTP_ADDR)")
	fs.StringVar(&cfg.GRPCAddr, "grpc-addr", envOr("PROTOMOCK_GRPC_ADDR", ":8086"), "gRPC mocks listen address (env PROTOMOCK_GRPC_ADDR)")
	fs.StringVar(&cfg.MocksDir, "mocks-dir", envOr("PROTOMOCK_MOCKS_DIR", "mocks"), "mocks root folder (env PROTOMOCK_MOCKS_DIR)")
	fs.StringVar(&cfg.WebDir, "web-dir", envOr("PROTOMOCK_WEB_DIR", ""), "serve UI files from this folder instead of the built-in copy, e.g. internal/web/ui (env PROTOMOCK_WEB_DIR)")

	reload, err := time.ParseDuration(envOr("PROTOMOCK_RELOAD_INTERVAL", "2s"))
	if err != nil {
		return Config{}, fmt.Errorf("PROTOMOCK_RELOAD_INTERVAL: %w", err)
	}
	fs.DurationVar(&cfg.ReloadInterval, "reload-interval", reload, "how often to check the mocks folder for changes and hot-reload; 0 disables (env PROTOMOCK_RELOAD_INTERVAL)")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
