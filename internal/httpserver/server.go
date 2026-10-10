package httpserver

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/avijeet7/protomock/internal/models"
)

func StartHTTPServer(routes []models.Route, uiHandler http.Handler) {
	mux := NewRouter(routes, uiHandler)
	log.Println("[HTTP] Server started on :8085")
	http.ListenAndServe(":8085", mux)
}

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
	mux.Handle("/static/", uiHandler)
	mux.Handle("/api/endpoints", uiHandler)
	mux.HandleFunc("/", makeMockHandler(urlToRoutes, regexRoutes))
	return mux
}

func isUIPath(path string) bool {
	return path == "/protomock-ui" || path == "/api/endpoints" || strings.HasPrefix(path, "/static/")
}

// partitionRoutes splits routes into exact-URL groups and regex routes.
// Regex patterns are compiled once here; invalid patterns are logged and skipped.
func partitionRoutes(routes []models.Route) (map[string][]models.Route, []models.Route) {
	urlToRoutes := make(map[string][]models.Route)
	var regexRoutes []models.Route

	for _, route := range routes {
		if !isRegexURL(route.URL) {
			urlToRoutes[route.URL] = append(urlToRoutes[route.URL], route)
			continue
		}

		re, err := regexp.Compile("^" + route.URL + "$")
		if err != nil {
			log.Printf("[HTTP] ❌ Invalid URL regex %q in %s: %v (stub skipped)", route.URL, route.FilePath, err)
			continue
		}
		route.PathRegex = re
		regexRoutes = append(regexRoutes, route)
	}
	return urlToRoutes, regexRoutes
}
