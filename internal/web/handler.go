package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/avijeet7/protomock/internal/models"
)

type uiData struct {
	HTTPEndpoints []httpEndpoint `json:"http_endpoints"`
	GRPCEndpoints []string       `json:"grpc_endpoints"`
}

type httpEndpoint struct {
	Method  string `json:"method"`
	URL     string `json:"url"`
	IsProto bool   `json:"is_proto"`
}

func NewUIHandler(httpRoutes []models.Route, grpcRoutes []models.Route, webDir string) http.Handler {
	mux := http.NewServeMux()

	// Serve static files
	staticDir := filepath.Join(webDir, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	// Serve the index.html file
	indexFile := filepath.Join(webDir, "templates", "index.html")
	mux.HandleFunc("/protomock-ui", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, indexFile)
	})

	// API endpoint to get the endpoint data
	mux.HandleFunc("/api/endpoints", func(w http.ResponseWriter, r *http.Request) {
		data := getEndpointData(httpRoutes, grpcRoutes)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	return mux
}

func getEndpointData(httpRoutes []models.Route, grpcRoutes []models.Route) uiData {
	// Prepare HTTP endpoints data
	httpEndpointsMap := make(map[string]httpEndpoint)
	for _, route := range httpRoutes {
		key := route.Method + route.URL
		if _, exists := httpEndpointsMap[key]; !exists {
			httpEndpointsMap[key] = httpEndpoint{
				Method:  route.Method,
				URL:     route.URL,
				IsProto: route.ProtoEncoded,
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
