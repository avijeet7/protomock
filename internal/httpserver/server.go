package httpserver

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/avijeet7/protomock/internal/match"
	"github.com/avijeet7/protomock/internal/models"
)

// Server serves HTTP mocks and the UI. Its routes can be replaced while it runs (hot reload).
type Server struct {
	ui     http.Handler
	router atomic.Pointer[routerHolder]
}

type routerHolder struct{ http.Handler }

// NewServer returns a server for routes and the UI handler.
func NewServer(routes []models.Route, uiHandler http.Handler) *Server {
	s := &Server{ui: uiHandler}
	s.Update(routes)
	return s
}

// Update atomically replaces the served routes; in-flight requests finish on the old ones.
func (s *Server) Update(routes []models.Route) {
	s.router.Store(&routerHolder{NewRouter(routes, s.ui)})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.Load().ServeHTTP(w, r)
}

// ListenAndServe serves on addr (e.g. ":8085"). It blocks until the server stops
// and returns why, e.g. the port is in use.
func (s *Server) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("HTTP server: %w", err)
	}
	log.Printf("[HTTP] Server started on %s", lis.Addr())
	return s.Serve(lis)
}

// Serve accepts connections on lis until it fails.
func (s *Server) Serve(lis net.Listener) error { return http.Serve(lis, s) }

// NewRouter builds the HTTP handler for all mock routes plus the web UI.
//
// Stub URLs are looked up in our own map rather than registered as ServeMux
// patterns, so a stub URL can't become an accidental prefix match ("/api/")
// or collide with the UI paths and panic.
func NewRouter(routes []models.Route, uiHandler http.Handler) http.Handler {
	urlToRoutes, regexRoutes := partitionRoutes(routes)

	for url, groupedRoutes := range urlToRoutes {
		if isUIPath(url) {
			log.Printf("[HTTP] ⚠️ Stub URL %s is reserved for the ProtoMock UI and will not be served", url)
			continue
		}
		protoCount := 0
		for _, route := range groupedRoutes {
			if route.ProtoEncoded {
				protoCount++
			}
		}
		log.Printf("[HTTP] Registered handler for URL: %s (Total: %d, Proto: %d, JSON: %d)", url, len(groupedRoutes), protoCount, len(groupedRoutes)-protoCount)
	}
	if len(regexRoutes) > 0 {
		log.Printf("[HTTP] Registered handler for %d regex routes", len(regexRoutes))
	}

	mux := http.NewServeMux()
	mux.Handle("/protomock-ui", uiHandler)
	mux.Handle("/protomock-ui/", uiHandler)
	mux.HandleFunc("/", makeMockHandler(urlToRoutes, regexRoutes))
	return mux
}

// isUIPath reports whether path belongs to the UI (/protomock-ui and everything under it).
func isUIPath(path string) bool {
	return path == "/protomock-ui" || strings.HasPrefix(path, "/protomock-ui/")
}

// partitionRoutes splits routes into exact-URL groups and regex routes.
// Regex patterns are compiled once here; invalid patterns are logged and skipped.
func partitionRoutes(routes []models.Route) (map[string][]models.Route, []models.Route) {
	urlToRoutes := make(map[string][]models.Route)
	var regexRoutes []models.Route

	for _, route := range routes {
		if route.PathRegex == nil && !match.IsRegexURL(route.URL) {
			urlToRoutes[route.URL] = append(urlToRoutes[route.URL], route)
			continue
		}

		if route.PathRegex == nil {
			re, err := match.CompileURL(route.URL)
			if err != nil {
				log.Printf("[HTTP] ❌ Invalid URL regex %q in %s: %v (stub skipped)", route.URL, route.FilePath, err)
				continue
			}
			route.PathRegex = re
		}
		regexRoutes = append(regexRoutes, route)
	}
	return urlToRoutes, regexRoutes
}
