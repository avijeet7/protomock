package httpserver

import (
	"log"
	"net/http"

	"github.com/avijeet7/protomock/internal/models"
)

func StartHTTPServer(routes []models.Route) {
	for _, route := range routes {
		log.Printf("[HTTP] Registered route: %s %s", route.Method, route.URL)
		http.HandleFunc(route.URL, makeHandler(route))
	}

	log.Println("[HTTP] Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
