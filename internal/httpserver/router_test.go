package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avijeet7/protomock/internal/models"
)

// fakeUI stands in for the real web UI so router tests don't depend on template files.
var fakeUI = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("ui"))
})

func TestNewRouter(t *testing.T) {
	exactOnly := []models.Route{
		{URL: "/users", Method: "GET", Status: 200, RawJSONBody: []byte(`"users"`)},
		{URL: "/api/", Method: "GET", Status: 200, RawJSONBody: []byte(`"api"`)},
	}
	withRegex := append(exactOnly[:len(exactOnly):len(exactOnly)],
		models.Route{URL: "/hello/[0-9]+/world", Method: "GET", Status: 200, RawJSONBody: []byte(`"regex"`)})

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string // checked only when non-empty
	}{
		{"root redirects to UI", "/", http.StatusFound, ""},
		{"UI page is served", "/protomock-ui", http.StatusOK, "ui"},
		{"UI API is served", "/protomock-ui/api/endpoints", http.StatusOK, "ui"},
		{"former UI path is free for mocks", "/api/endpoints", http.StatusNotFound, ""},
		{"exact mock", "/users", http.StatusOK, `"users"`},
		{"trailing-slash stub is exact, not a prefix", "/api/anything", http.StatusNotFound, ""},
		{"unknown path is 404", "/nope", http.StatusNotFound, ""},
	}

	for _, set := range []struct {
		name   string
		routes []models.Route
	}{{"exact routes only", exactOnly}, {"with regex routes", withRegex}} {
		router := NewRouter(set.routes, fakeUI)

		for _, tt := range tests {
			t.Run(set.name+"/"+tt.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

				if rec.Code != tt.wantStatus {
					t.Fatalf("GET %s status = %d, want %d", tt.path, rec.Code, tt.wantStatus)
				}
				if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
					t.Errorf("GET %s body = %q, want %q", tt.path, rec.Body.String(), tt.wantBody)
				}
			})
		}
	}

	t.Run("regex mock", func(t *testing.T) {
		rec := httptest.NewRecorder()
		NewRouter(withRegex, fakeUI).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hello/42/world", nil))
		if rec.Body.String() != `"regex"` {
			t.Errorf("body = %q, want %q", rec.Body.String(), `"regex"`)
		}
	})
}

func TestNewRouter_StubOnReservedPathDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewRouter panicked: %v", r)
		}
	}()
	NewRouter([]models.Route{
		{URL: "/", Method: "GET", Status: 200},
		{URL: "/protomock-ui", Method: "GET", Status: 200},
	}, fakeUI)
}
