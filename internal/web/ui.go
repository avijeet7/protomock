// Package web serves the ProtoMock UI and its JSON API under /protomock-ui/.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"

	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/store"
)

// Backend is what the UI needs from the running app.
type Backend interface {
	Set() *loader.Set                            // currently loaded mocks
	MocksDir() string                            // folder mocks are loaded from
	Locked(fn func() error) (*loader.Set, error) // run a file edit, then reload, without interleaving
	Reload() (*loader.Set, error)                // re-read the mocks folder
	ServeMock(w http.ResponseWriter, r *http.Request)
	GRPCTarget() string // address to dial the gRPC mock server
}

// assets are the UI files compiled into the binary.
//
//go:embed ui
var assets embed.FS

// NewUIHandler serves the UI and API under /protomock-ui. If assetsDir is set, UI files
// are read from that folder instead of the embedded copy (handy while developing the UI).
func NewUIHandler(backend Backend, assetsDir string) http.Handler {
	var files fs.FS
	if assetsDir != "" {
		files = os.DirFS(assetsDir)
	} else {
		files, _ = fs.Sub(assets, "ui")
	}

	api := &api{backend: backend, store: &store.Store{Root: backend.MocksDir()}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /protomock-ui", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, files, "index.html")
	})
	mux.Handle("GET /protomock-ui/", http.StripPrefix("/protomock-ui/", http.FileServerFS(files)))
	api.register(mux, "/protomock-ui/api")
	return mux
}
