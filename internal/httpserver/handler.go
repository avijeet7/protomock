package httpserver

import (
	"log"
	"net/http"

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
					data, err := route.Message.MarshalJSON()
					if err != nil {
						http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
						return
					}
					w.Write(data)
				}
				return
			}
		}

		http.NotFound(w, r)
	}
}
