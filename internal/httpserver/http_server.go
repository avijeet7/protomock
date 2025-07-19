package httpserver

import (
	"log"
	"net/http"

	"github.com/avijeet7/protomock/internal/models"
)

func StartHTTPServer(routes []models.Route) {
	for _, route := range routes {
		log.Printf("[HTTP] Registered route: %s", route.URL)
		http.HandleFunc(route.URL, makeHandler(route))
	}

	log.Println("[HTTP] Server started on :8080")
	http.ListenAndServe(":8080", nil)
}

func makeHandler(route models.Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[HTTP] Serving mock for %s", route.URL)

		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(route.Status)

		msg := route.Message
		if msg == nil {
			http.Error(w, "No message loaded", http.StatusInternalServerError)
			return
		}

		data, err := msg.Marshal()
		if err != nil {
			http.Error(w, "Failed to marshal Protobuf", http.StatusInternalServerError)
			return
		}

		w.Write(data)
	}
}
