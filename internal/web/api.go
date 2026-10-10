package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/store"
)

// maxBody limits request bodies (stubs, protos, try-it requests).
const maxBody = 5 << 20

type api struct {
	backend Backend
	store   *store.Store
}

// register adds the API routes under prefix (e.g. "/protomock-ui/api").
//
// Mock and file names are passed as query parameters (?kind=&mock=&file=)
// because mock names may contain slashes ("team/users").
func (a *api) register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/health", a.health)
	mux.HandleFunc("GET "+prefix+"/mocks", a.listMocks)
	mux.HandleFunc("POST "+prefix+"/mocks", a.createMock)
	mux.HandleFunc("GET "+prefix+"/template", a.template)
	mux.HandleFunc("POST "+prefix+"/validate", a.validate)
	mux.HandleFunc("PUT "+prefix+"/stubs", a.saveStub)
	mux.HandleFunc("DELETE "+prefix+"/stubs", a.deleteStub)
	mux.HandleFunc("PUT "+prefix+"/protos", a.saveProto)
	mux.HandleFunc("DELETE "+prefix+"/protos", a.deleteProto)
	mux.HandleFunc("POST "+prefix+"/reload", a.reload)
	mux.HandleFunc("POST "+prefix+"/try/http", a.tryHTTP)
	mux.HandleFunc("POST "+prefix+"/try/grpc", a.tryGRPC)
}

// GET /health — 200 while ProtoMock is serving; 503 in strict mode while any mock file
// fails to load. Used by `protomock healthcheck`, Docker HEALTHCHECK and compose healthchecks.
func (a *api) health(w http.ResponseWriter, r *http.Request) {
	set := a.backend.Set()
	healthy := !a.backend.Strict() || len(set.Problems) == 0
	status, code := "ok", http.StatusOK
	if !healthy {
		status, code = "unhealthy", http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{
		"status":     status,
		"strict":     a.backend.Strict(),
		"mocks":      len(set.Mocks),
		"http_stubs": len(set.HTTP),
		"grpc_stubs": len(set.GRPC),
		"problems":   len(set.Problems),
	})
}

// GET /mocks — every mock folder with its protos, methods, stubs and problems.
func (a *api) listMocks(w http.ResponseWriter, r *http.Request) {
	a.writeMocks(w, a.backend.Set())
}

func (a *api) writeMocks(w http.ResponseWriter, set *loader.Set) {
	writeJSON(w, http.StatusOK, newMocksView(set, a.backend.MocksDir(), a.store.Writable()))
}

// POST /mocks {kind, name, proto_file?, proto?} — create a mock folder, optionally with a .proto.
func (a *api) createMock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind      loader.Kind `json:"kind"`
		Name      string      `json:"name"`
		ProtoFile string      `json:"proto_file"`
		Proto     string      `json:"proto"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	dir, err := a.store.MockDir(req.Kind, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	var protoPath string
	if req.Proto != "" {
		if req.ProtoFile == "" {
			req.ProtoFile = filepath.Base(req.Name) + ".proto"
		}
		if protoPath, err = a.store.ProtoPath(req.Kind, req.Name, req.ProtoFile); err != nil {
			writeError(w, err)
			return
		}
		if _, err := loader.CompileProtosWith(dir, []string{req.ProtoFile}, map[string][]byte{req.ProtoFile: []byte(req.Proto)}); err != nil {
			writeErrorf(w, http.StatusBadRequest, "proto doesn't compile: %v", err)
			return
		}
	}

	set, err := a.backend.Locked(func() error {
		if err := a.store.CreateMock(req.Kind, req.Name); err != nil {
			return err
		}
		if protoPath != "" {
			return store.WriteFile(protoPath, []byte(req.Proto))
		}
		return nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	log.Printf("[UI] ➕ Created mock %s/%s", req.Kind, req.Name)
	a.writeMocks(w, set)
}

// GET /template?kind=&mock=&message= — example JSON body for a protobuf message type.
func (a *api) template(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mock := a.findMock(loader.Kind(q.Get("kind")), q.Get("mock"))
	if mock == nil {
		writeErrorf(w, http.StatusNotFound, "mock %s/%s not found", q.Get("kind"), q.Get("mock"))
		return
	}
	md := mock.Protos.Message(q.Get("message"))
	if md == nil {
		writeErrorf(w, http.StatusNotFound, "message type %q not found in mock %s", q.Get("message"), mock.Name)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(loader.Template(md))
}

// POST /validate {kind, mock, content} — check a stub without saving it.
func (a *api) validate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    loader.Kind `json:"kind"`
		Mock    string      `json:"mock"`
		Content string      `json:"content"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	dir, err := a.store.MockDir(req.Kind, req.Mock)
	if err != nil {
		writeError(w, err)
		return
	}
	mock := loader.LoadMock(a.backend.MocksDir(), req.Kind, req.Mock)
	if _, err := mock.BuildRoute([]byte(req.Content), filepath.Join(dir, "stubs")); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}

// PUT /stubs?kind=&mock=&file= (body: stub JSON) — validate and save a stub, then reload.
func (a *api) saveStub(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind, mockName, file := loader.Kind(q.Get("kind")), q.Get("mock"), q.Get("file")
	path, err := a.store.StubPath(kind, mockName, file)
	if err != nil {
		writeError(w, err)
		return
	}
	content, ok := readBody(w, r)
	if !ok {
		return
	}
	if _, err := os.Stat(filepath.Dir(filepath.Dir(path))); err != nil {
		writeErrorf(w, http.StatusNotFound, "mock %s/%s not found", kind, mockName)
		return
	}
	mock := loader.LoadMock(a.backend.MocksDir(), kind, mockName)
	if _, err := mock.BuildRoute(content, path); err != nil {
		writeErrorf(w, http.StatusBadRequest, "invalid stub: %v", err)
		return
	}

	set, err := a.backend.Locked(func() error { return store.WriteFile(path, content) })
	if err != nil {
		writeError(w, err)
		return
	}
	log.Printf("[UI] 💾 Saved %s", relPath(a.backend.MocksDir(), path))
	a.writeMocks(w, set)
}

// DELETE /stubs?kind=&mock=&file= — delete a stub, then reload.
func (a *api) deleteStub(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path, err := a.store.StubPath(loader.Kind(q.Get("kind")), q.Get("mock"), q.Get("file"))
	if err != nil {
		writeError(w, err)
		return
	}
	a.deleteFile(w, path)
}

// PUT /protos?kind=&mock=&file= (body: .proto source) — check it compiles, save, reload.
func (a *api) saveProto(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind, mockName, file := loader.Kind(q.Get("kind")), q.Get("mock"), q.Get("file")
	path, err := a.store.ProtoPath(kind, mockName, file)
	if err != nil {
		writeError(w, err)
		return
	}
	content, ok := readBody(w, r)
	if !ok {
		return
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		writeErrorf(w, http.StatusNotFound, "mock %s/%s not found", kind, mockName)
		return
	}
	names := []string{file}
	if mock := a.findMock(kind, mockName); mock != nil {
		names = mock.ProtoFiles
		if !contains(names, file) {
			names = append(names, file)
		}
	}
	if _, err := loader.CompileProtosWith(dir, names, map[string][]byte{file: content}); err != nil {
		writeErrorf(w, http.StatusBadRequest, "proto doesn't compile: %v", err)
		return
	}

	set, err := a.backend.Locked(func() error { return store.WriteFile(path, content) })
	if err != nil {
		writeError(w, err)
		return
	}
	log.Printf("[UI] 💾 Saved %s", relPath(a.backend.MocksDir(), path))
	a.writeMocks(w, set)
}

// DELETE /protos?kind=&mock=&file= — delete a .proto file, then reload.
func (a *api) deleteProto(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path, err := a.store.ProtoPath(loader.Kind(q.Get("kind")), q.Get("mock"), q.Get("file"))
	if err != nil {
		writeError(w, err)
		return
	}
	a.deleteFile(w, path)
}

func (a *api) deleteFile(w http.ResponseWriter, path string) {
	set, err := a.backend.Locked(func() error { return store.Delete(path) })
	if err != nil {
		writeError(w, err)
		return
	}
	log.Printf("[UI] 🗑 Deleted %s", relPath(a.backend.MocksDir(), path))
	a.writeMocks(w, set)
}

// POST /reload — re-read the mocks folder now.
func (a *api) reload(w http.ResponseWriter, r *http.Request) {
	set, err := a.backend.Reload()
	if err != nil {
		writeError(w, err)
		return
	}
	a.writeMocks(w, set)
}

func (a *api) findMock(kind loader.Kind, name string) *loader.Mock {
	for _, m := range a.backend.Set().Mocks {
		if m.Kind == kind && m.Name == name {
			return m
		}
	}
	return nil
}

// --- helpers ---

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeErrorf(w, http.StatusBadRequest, "reading request: %v", err)
		return nil, false
	}
	return body, true
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	body, ok := readBody(w, r)
	if !ok {
		return false
	}
	if err := json.Unmarshal(body, v); err != nil {
		writeErrorf(w, http.StatusBadRequest, "invalid JSON: %v", err)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[UI] Failed to write response: %v", err)
	}
}

func writeErrorf(w http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(w, status, map[string]string{"error": fmt.Sprintf(format, args...)})
}

// writeError maps an error to an HTTP status: bad names -> 400, missing files -> 404,
// read-only mocks folder -> 403, anything else -> 500.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, store.ErrInvalidName):
		status = http.StatusBadRequest
	case errors.Is(err, fs.ErrNotExist):
		status = http.StatusNotFound
	case errors.Is(err, fs.ErrPermission):
		status = http.StatusForbidden
		err = fmt.Errorf("%w (is the mocks folder mounted read-only?)", err)
	case errors.Is(err, fs.ErrExist):
		status = http.StatusConflict
	}
	writeErrorf(w, status, "%v", err)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
