package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avijeet7/protomock/internal/models"
)

func TestMatchBody(t *testing.T) {
	tests := []struct {
		name     string
		expected string // stub's request.body
		actual   string // incoming request body
		want     bool
	}{
		{"no body match configured", ``, `{"a":1}`, true},
		{"exact match", `{"user":"bob"}`, `{"user":"bob"}`, true},
		{"extra request fields are ignored", `{"user":"bob"}`, `{"user":"bob","remember":true}`, true},
		{"different value", `{"user":"bob"}`, `{"user":"alice"}`, false},
		{"missing field", `{"user":"bob"}`, `{"name":"bob"}`, false},
		{"nested partial match", `{"user":{"id":1}}`, `{"user":{"id":1,"name":"bob"}}`, true},
		{"nested mismatch", `{"user":{"id":1}}`, `{"user":{"id":2}}`, false},
		{"arrays must be equal", `{"tags":["a","b"]}`, `{"tags":["a","b"]}`, true},
		{"array mismatch", `{"tags":["a","b"]}`, `{"tags":["a"]}`, false},
		{"number types compare by value", `{"n":1}`, `{"n":1.0}`, true},
		{"invalid request JSON never matches", `{"user":"bob"}`, `not json`, false},
		{"empty request body does not match", `{"user":"bob"}`, ``, false},
		{"non-object stub vs invalid request", `"x"`, `not json`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := models.Route{BodyMatch: []byte(tt.expected)}
			if got := matchBody([]byte(tt.actual), route); got != tt.want {
				t.Errorf("matchBody(expected=%s, actual=%s) = %v, want %v", tt.expected, tt.actual, got, tt.want)
			}
		})
	}
}

func BenchmarkMatchPathRegex(b *testing.B) {
	_, regexRoutes := partitionRoutes([]models.Route{{URL: "/hello/[0-9]+/world", Method: "GET"}})
	route := regexRoutes[0]
	req := httptest.NewRequest(http.MethodGet, "/hello/123/world", nil)

	for b.Loop() {
		matchPath(req, route)
	}
}

func TestPartitionRoutes(t *testing.T) {
	exact, regexRoutes := partitionRoutes([]models.Route{
		{URL: "/users", Method: "GET"},
		{URL: "/hello/[0-9]+/world", Method: "GET"},
		{URL: "/broken/[0-9", Method: "GET"}, // invalid regex: unclosed bracket
	})

	if len(exact["/users"]) != 1 {
		t.Errorf("exact routes = %v, want /users", exact)
	}
	if len(regexRoutes) != 1 {
		t.Fatalf("got %d regex routes, want 1 (invalid pattern must be skipped)", len(regexRoutes))
	}
	if regexRoutes[0].PathRegex == nil {
		t.Fatal("regex route was not precompiled")
	}
}

func TestMatchPath(t *testing.T) {
	_, regexRoutes := partitionRoutes([]models.Route{{URL: "/hello/[0-9]+/world"}})
	route := regexRoutes[0]

	tests := []struct {
		path string
		want bool
	}{
		{"/hello/123/world", true},
		{"/hello/abc/world", false},
		{"/hello/123/world/extra", false}, // anchored: whole path must match
		{"/prefix/hello/123/world", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if got := matchPath(req, route); got != tt.want {
				t.Errorf("matchPath(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
