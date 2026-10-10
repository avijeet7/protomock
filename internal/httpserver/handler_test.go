package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avijeet7/protomock/internal/models"
)

func TestGroupedHandler_SetsJSONContentType(t *testing.T) {
	routes := []models.Route{{
		URL:         "/users",
		Method:      http.MethodGet,
		Status:      http.StatusCreated,
		RawJSONBody: []byte(`{"id":1}`),
	}}

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()
	makeGroupedHandler(routes)(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	// Result().Header reflects what was actually sent to the client;
	// rec.Header() would also include headers set after WriteHeader.
	if got := rec.Result().Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if got := rec.Body.String(); got != `{"id":1}` {
		t.Errorf("body = %q, want %q", got, `{"id":1}`)
	}
}

func TestGroupedHandler_BodyMatchOnLaterStub(t *testing.T) {
	routes := []models.Route{
		{
			URL:         "/login",
			Method:      http.MethodPost,
			BodyMatch:   []byte(`{"user":"alice"}`),
			Status:      http.StatusOK,
			RawJSONBody: []byte(`{"who":"alice"}`),
		},
		{
			URL:         "/login",
			Method:      http.MethodPost,
			BodyMatch:   []byte(`{"user":"bob"}`),
			Status:      http.StatusOK,
			RawJSONBody: []byte(`{"who":"bob"}`),
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"user":"bob"}`))
	rec := httptest.NewRecorder()
	makeGroupedHandler(routes)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (second stub should have matched)", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != `{"who":"bob"}` {
		t.Errorf("body = %q, want %q", got, `{"who":"bob"}`)
	}
}
