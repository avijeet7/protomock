package httpserver

import (
	"log"
	"net/http"

	"github.com/avijeet7/protomock/internal/models"
)

func StartHTTPServer(routes []models.Route) {
	urlToRoutes := make(map[string][]models.Route)

	for _, route := range routes {
		urlToRoutes[route.URL] = append(urlToRoutes[route.URL], route)
	}

	for url, groupedRoutes := range urlToRoutes {
		http.HandleFunc(url, makeGroupedHandler(groupedRoutes))
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

	log.Println("[HTTP] Server started on :8085")
	http.ListenAndServe(":8085", nil)
}
