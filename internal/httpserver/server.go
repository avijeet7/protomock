package httpserver

import (
	"log"
	"net/http"

	"github.com/avijeet7/protomock/internal/models"
)

func StartHTTPServer(routes []models.Route, uiHandler http.Handler) {
	urlToRoutes := make(map[string][]models.Route)

	for _, route := range routes {
		urlToRoutes[route.URL] = append(urlToRoutes[route.URL], route)
	}

	mux := http.NewServeMux()

	for url, groupedRoutes := range urlToRoutes {
		mux.HandleFunc(url, makeGroupedHandler(groupedRoutes))
		protoCount := 0
		jsonCount := 0
		for _, route := range groupedRoutes {
			if route.ProtoEncoded {
				protoCount++
			} else {
				jsonCount++
			}
		}
		log.Printf("[HTTP] Registered handler for URL: %s (Total: %d, Proto: %d, JSON: %d)", url, len(groupedRoutes), protoCount, jsonCount)
	}

	mux.Handle("/protomock-ui", uiHandler)
	mux.Handle("/static/", uiHandler)
	mux.Handle("/api/endpoints", uiHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		// Redirect root to the UI page
		http.Redirect(w, r, "/protomock-ui", http.StatusFound)
	})

	log.Println("[HTTP] Server started on :8085")
	http.ListenAndServe(":8085", mux)
}
