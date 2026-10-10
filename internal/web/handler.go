package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sort"

	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/models"
)

type uiData struct {
	HTTPEndpoints []httpEndpoint `json:"http_endpoints"`
	GRPCEndpoints []string       `json:"grpc_endpoints"`
}

type httpEndpoint struct {
	Method      string          `json:"method"`
	URL         string          `json:"url"`
	IsProto     bool            `json:"is_proto"`
	HeaderMatch json.RawMessage `json:"header_match,omitempty"`
	BodyMatch   json.RawMessage `json:"body_match,omitempty"`
}

// assets are the UI files compiled into the binary.
//
//go:embed ui
var assets embed.FS

// NewUIHandler serves the UI under /protomock-ui. If assetsDir is set, UI files are read
// from that folder instead of the embedded copy (handy while developing the UI).
// Backend is what the UI needs from the running app.
type Backend interface {
	Set() *loader.Set // currently loaded mocks
}

func NewUIHandler(backend Backend, assetsDir string) http.Handler {
	var files fs.FS
	if assetsDir != "" {
		files = os.DirFS(assetsDir)
	} else {
		files, _ = fs.Sub(assets, "ui")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /protomock-ui", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, files, "index.html")
	})
	mux.Handle("GET /protomock-ui/", http.StripPrefix("/protomock-ui/", http.FileServerFS(files)))

	mux.HandleFunc("GET /protomock-ui/api/endpoints", func(w http.ResponseWriter, r *http.Request) {
		set := backend.Set()
		data := getEndpointData(set.HTTP, set.GRPC)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("[UI] Failed to write endpoint data: %v", err)
		}
	})

	return mux
}

func getEndpointData(httpRoutes []models.Route, grpcRoutes []models.Route) uiData {
	// Prepare HTTP endpoints data
	httpEndpointsMap := make(map[string]httpEndpoint)
	for _, route := range httpRoutes {
		key := route.FilePath // Use FilePath as a unique key
		if _, exists := httpEndpointsMap[key]; !exists {
			headerMatch, _ := json.Marshal(route.HeaderMatch)
			bodyMatch, _ := json.Marshal(route.BodyMatch)

			httpEndpointsMap[key] = httpEndpoint{
				Method:      route.Method,
				URL:         route.URL,
				IsProto:     route.ProtoEncoded,
				HeaderMatch: headerMatch,
				BodyMatch:   bodyMatch,
			}
		}
	}

	var displayHTTPEndpoints []httpEndpoint
	for _, ep := range httpEndpointsMap {
		displayHTTPEndpoints = append(displayHTTPEndpoints, ep)
	}
	sort.Slice(displayHTTPEndpoints, func(i, j int) bool {
		if displayHTTPEndpoints[i].URL != displayHTTPEndpoints[j].URL {
			return displayHTTPEndpoints[i].URL < displayHTTPEndpoints[j].URL
		}
		return displayHTTPEndpoints[i].Method < displayHTTPEndpoints[j].Method
	})

	// Prepare gRPC endpoints data
	grpcEndpointsMap := make(map[string]struct{})
	for _, route := range grpcRoutes {
		grpcEndpointsMap[route.URL] = struct{}{}
	}

	var displayGRPCEndpoints []string
	for method := range grpcEndpointsMap {
		displayGRPCEndpoints = append(displayGRPCEndpoints, method)
	}
	sort.Strings(displayGRPCEndpoints)

	// Final data for the template
	return uiData{
		HTTPEndpoints: displayHTTPEndpoints,
		GRPCEndpoints: displayGRPCEndpoints,
	}
}
