package httpserver

import (
	"log"
	"net/http"
	"strings"

	"github.com/avijeet7/protomock/internal/models"
)

// makeGroupedHandler returns an HTTP handler for all stubs under the same URL.
func makeGroupedHandler(routes []models.Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, route := range routes {
			if matchMethod(r, route) && matchHeaders(r, route) && matchBody(r, route) {
				log.Printf("[HTTP] ✅ Serving mock for %s %s", r.Method, r.URL.Path)
				w.WriteHeader(route.Status)

				if route.ProtoEncoded {
					w.Header().Set("Content-Type", "application/x-protobuf")
					data, err := route.Message.Marshal()
					if err != nil {
						http.Error(w, "Failed to marshal Protobuf", http.StatusInternalServerError)
						return
					}
					w.Write(data)
				} else {
					w.Header().Set("Content-Type", "application/json")
					w.Write(route.RawJSONBody)
				}
				return
			}
		}

		http.NotFound(w, r)
	}
}

// makeRegexHandler returns an HTTP handler for all regex routes.
func makeRegexHandler(regexRoutes []models.Route, urlToRoutes map[string][]models.Route, uiHandler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// First, check if there is an exact match
		if routes, ok := urlToRoutes[r.URL.Path]; ok {
			makeGroupedHandler(routes)(w, r)
			return
		}

		// If no exact match, check for regex match
		for _, route := range regexRoutes {
			if matchPath(r, route) && matchMethod(r, route) && matchHeaders(r, route) && matchBody(r, route) {
				log.Printf("[HTTP] ✅ Serving mock for %s %s", r.Method, r.URL.Path)
				w.WriteHeader(route.Status)

				if route.ProtoEncoded {
					w.Header().Set("Content-Type", "application/x-protobuf")
					data, err := route.Message.Marshal()
					if err != nil {
						http.Error(w, "Failed to marshal Protobuf", http.StatusInternalServerError)
						return
					}
					w.Write(data)
				} else {
					w.Header().Set("Content-Type", "application/json")
					w.Write(route.RawJSONBody)
				}
				return
			}
		}

		// If no mock matches, check if it's a UI path
		if strings.HasPrefix(r.URL.Path, "/protomock-ui") || strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/api/endpoints" {
			uiHandler.ServeHTTP(w, r)
			return
		}

		// If it's not a mock and not a UI path, then it's a 404
		http.NotFound(w, r)
	}
}
