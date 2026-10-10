//go:build bench

// Package benchmark measures request-matching performance through the public HTTP router.
package benchmark

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avijeet7/protomock/internal/httpserver"
	"github.com/avijeet7/protomock/internal/models"
)

var noUI = http.NotFoundHandler()

func BenchmarkRouter_ExactRoute(b *testing.B) {
	router := httpserver.NewRouter([]models.Route{
		{URL: "/users", Method: "GET", Status: 200, RawJSONBody: []byte(`{}`)},
	}, noUI)
	req := httptest.NewRequest(http.MethodGet, "/users", nil)

	for b.Loop() {
		router.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkRouter_RegexRoute(b *testing.B) {
	router := httpserver.NewRouter([]models.Route{
		{URL: "/hello/[0-9]+/world", Method: "GET", Status: 200, RawJSONBody: []byte(`{}`)},
	}, noUI)
	req := httptest.NewRequest(http.MethodGet, "/hello/123/world", nil)

	for b.Loop() {
		router.ServeHTTP(httptest.NewRecorder(), req)
	}
}
