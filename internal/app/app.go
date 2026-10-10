// Package app wires ProtoMock together: it loads the mocks folder, runs the
// HTTP (mocks + UI) and gRPC servers, and hot-reloads mocks when they change.
package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/avijeet7/protomock/internal/config"
	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/httpserver"
	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/web"
)

// App is a running ProtoMock instance.
type App struct {
	cfg  config.Config
	http *httpserver.Server
	grpc *grpcserver.Server

	mu          sync.Mutex // serializes reloads (and edits made through the UI)
	set         atomic.Pointer[loader.Set]
	fingerprint string // of the mocks folder at the last reload
}

// New loads the mocks folder and prepares the servers (without starting them).
func New(cfg config.Config) (*App, error) {
	a := &App{cfg: cfg}
	set, fingerprint, err := a.load()
	if err != nil {
		return nil, err
	}
	a.set.Store(set)
	a.fingerprint = fingerprint

	a.http = httpserver.NewServer(set.HTTP, web.NewUIHandler(a, cfg.WebDir))
	a.grpc = grpcserver.NewServer(set.GRPC)
	return a, nil
}

// Run listens on the configured addresses and serves until a server fails
// (e.g. its port is in use).
func (a *App) Run() error {
	httpLis, err := net.Listen("tcp", a.cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("HTTP server: %w", err)
	}
	grpcLis, err := net.Listen("tcp", a.cfg.GRPCAddr)
	if err != nil {
		_ = httpLis.Close()
		return fmt.Errorf("gRPC server: %w", err)
	}
	log.Printf("🌐 ProtoMock UI available at %s/protomock-ui", baseURL(a.cfg.HTTPAddr))
	return a.Serve(httpLis, grpcLis)
}

// Serve serves on the given listeners until a server fails.
// Both servers always run: the HTTP server also serves the UI, and gRPC mocks
// can be added later through the UI or on disk.
func (a *App) Serve(httpLis, grpcLis net.Listener) error {
	log.Printf("[HTTP] Server started on %s", httpLis.Addr())
	log.Printf("🚀 gRPC server started on %s", grpcLis.Addr())

	errCh := make(chan error, 2)
	go func() { errCh <- a.http.Serve(httpLis) }()
	go func() { errCh <- a.grpc.Serve(grpcLis) }()

	if a.cfg.ReloadInterval > 0 {
		go a.watch(a.cfg.ReloadInterval)
	}
	return <-errCh
}

// MocksDir is the folder mocks are loaded from.
func (a *App) MocksDir() string { return a.cfg.MocksDir }

// Set returns the currently loaded mocks.
func (a *App) Set() *loader.Set { return a.set.Load() }

// Reload re-reads the mocks folder and swaps the servers' routes.
func (a *App) Reload() (*loader.Set, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reloadLocked()
}

// Locked runs fn while holding the reload lock, then reloads. The UI uses it so a
// file edit and the reload that applies it can't interleave with another edit.
func (a *App) Locked(fn func() error) (*loader.Set, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := fn(); err != nil {
		return nil, err
	}
	return a.reloadLocked()
}

func (a *App) reloadLocked() (*loader.Set, error) {
	set, fingerprint, err := a.load()
	if err != nil {
		return nil, err
	}
	a.http.Update(set.HTTP)
	a.grpc.Update(set.GRPC)
	a.set.Store(set)
	a.fingerprint = fingerprint
	return set, nil
}

func (a *App) load() (*loader.Set, string, error) {
	fingerprint := fingerprintDir(a.cfg.MocksDir)
	set, err := loader.LoadAll(a.cfg.MocksDir)
	if err != nil {
		return nil, "", fmt.Errorf("loading mocks from %s: %w", a.cfg.MocksDir, err)
	}
	log.Printf("📦 Loaded %d mocks: %d HTTP stubs, %d gRPC stubs", len(set.Mocks), len(set.HTTP), len(set.GRPC))
	for _, p := range set.Problems {
		log.Printf("⚠️ Skipped %s: %s", p.Path, p.Error)
	}
	return set, fingerprint, nil
}

// watch reloads whenever files in the mocks folder change (checked every interval).
// Polling a small folder is cheap and needs no OS-specific file watching.
func (a *App) watch(interval time.Duration) {
	for range time.Tick(interval) {
		current := fingerprintDir(a.cfg.MocksDir)
		a.mu.Lock()
		changed := current != a.fingerprint
		a.mu.Unlock()
		if changed {
			log.Printf("🔄 Change detected in %s, reloading", a.cfg.MocksDir)
			if _, err := a.Reload(); err != nil {
				log.Printf("❌ Reload failed: %v", err)
			}
		}
	}
}

// fingerprintDir summarizes every file's path, size and modification time under dir.
func fingerprintDir(dir string) string {
	h := sha256.New()
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		_, _ = fmt.Fprintf(h, "%s|%d|%d|%t\n", path, info.Size(), info.ModTime().UnixNano(), d.IsDir()) // hash writes never fail
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}

// baseURL turns a listen address like ":8085" into a browsable base URL.
func baseURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	return "http://" + addr
}
