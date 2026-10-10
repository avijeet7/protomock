package httpserver

import (
	"log"
	"net/http"

	"github.com/avijeet7/protomock/internal/models"
)

// makeGroupedHandler returns an HTTP handler for all stubs under the same URL.
func makeGroupedHandler(routes []models.Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := readBody(r)
		for _, route := range routes {
			if matchMethod(r, route) && matchHeaders(r, route) && matchBody(body, route) {
				log.Printf("[HTTP] ✅ Serving mock for %s %s", r.Method, r.URL.Path)
				writeResponse(w, route)
				return
			}
		}

		http.NotFound(w, r)
	}
}

// makeMockHandler serves every non-UI request: exact stubs first, then regex
// stubs, then the root redirect to the UI, otherwise 404.
func makeMockHandler(urlToRoutes map[string][]models.Route, regexRoutes []models.Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if routes, ok := urlToRoutes[r.URL.Path]; ok {
			makeGroupedHandler(routes)(w, r)
			return
		}

		body := readBody(r)
		for _, route := range regexRoutes {
			if matchPath(r, route) && matchMethod(r, route) && matchHeaders(r, route) && matchBody(body, route) {
				log.Printf("[HTTP] ✅ Serving mock for %s %s", r.Method, r.URL.Path)
				writeResponse(w, route)
				return
			}
		}

		if r.URL.Path == "/" {
			http.Redirect(w, r, "/protomock-ui", http.StatusFound)
			return
		}

		http.NotFound(w, r)
	}
}

// writeResponse writes the stubbed response for a matched route.
// Headers must be set before WriteHeader, and the body is encoded first so
// an encoding failure can still be reported as a 500.
func writeResponse(w http.ResponseWriter, route models.Route) {
	contentType := "application/json"
	body := route.RawJSONBody

	if route.ProtoEncoded {
		data, err := route.Message.Marshal()
		if err != nil {
			http.Error(w, "Failed to marshal Protobuf", http.StatusInternalServerError)
			return
		}
		contentType = "application/x-protobuf"
		body = data
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(route.Status)
	_, _ = w.Write(body) // a write error means the client went away; nothing to do
}
