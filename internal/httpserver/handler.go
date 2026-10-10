package httpserver

import (
	"log"
	"net/http"

	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/protobuf/proto"
)

// makeGroupedHandler returns an HTTP handler for all stubs under the same URL.
func makeGroupedHandler(routes []models.Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := readBody(r)
		if route, ok := firstMatch(r, body, routes); ok {
			serve(w, r, route)
			return
		}
		writeNoMatch(w, r, body, routes, nil)
	}
}

// makeMockHandler serves every non-UI request: exact-URL stubs first, then regex
// stubs, then the root redirect to the UI, otherwise a 404 explaining the closest stubs.
func makeMockHandler(urlToRoutes map[string][]models.Route, regexRoutes []models.Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := readBody(r)

		exact := urlToRoutes[r.URL.Path]
		if route, ok := firstMatch(r, body, exact); ok {
			serve(w, r, route)
			return
		}

		var regexCandidates []models.Route
		for _, route := range regexRoutes {
			if matchPath(r, route) {
				regexCandidates = append(regexCandidates, route)
			}
		}
		if route, ok := firstMatch(r, body, regexCandidates); ok {
			serve(w, r, route)
			return
		}

		candidates := append(append([]models.Route{}, exact...), regexCandidates...)
		if r.URL.Path == "/" && len(candidates) == 0 {
			http.Redirect(w, r, "/protomock-ui", http.StatusFound)
			return
		}
		writeNoMatch(w, r, body, candidates, func() []models.Route { return similarRoutes(r, urlToRoutes, regexRoutes) })
	}
}

// firstMatch returns the first route (they're in priority order) matching method, headers and body.
func firstMatch(r *http.Request, body []byte, routes []models.Route) (models.Route, bool) {
	for _, route := range routes {
		if matchMethod(r, route) && matchHeaders(r, route) && matchBody(body, route) {
			return route, true
		}
	}
	return models.Route{}, false
}

func serve(w http.ResponseWriter, r *http.Request, route models.Route) {
	log.Printf("[HTTP] ✅ Serving %s for %s %s", route.Name(), r.Method, r.URL.Path)
	writeResponse(w, r, route)
}

// writeResponse writes the stubbed response for a matched route.
// Headers must be set before WriteHeader, and the body is encoded first so
// an encoding failure can still be reported as a 500.
func writeResponse(w http.ResponseWriter, r *http.Request, route models.Route) {
	if !route.WaitDelay(r.Context()) {
		return // client gave up while we were delaying
	}

	contentType := "application/json"
	body := route.RawJSONBody

	if route.ProtoEncoded {
		data, err := proto.Marshal(route.Message)
		if err != nil {
			http.Error(w, "Failed to marshal Protobuf", http.StatusInternalServerError)
			return
		}
		contentType = "application/x-protobuf"
		body = data
	}

	w.Header().Set("Content-Type", contentType)
	for key, value := range route.Headers {
		w.Header().Set(key, value) // stub headers may override the default Content-Type
	}
	w.WriteHeader(route.Status)
	_, _ = w.Write(body) // a write error means the client went away; nothing to do
}
