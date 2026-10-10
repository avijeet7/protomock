package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avijeet7/protomock/internal/models"
)

func noMatchRoutes() []models.Route {
	return []models.Route{
		{Mock: "users", FilePath: "/m/http/users/stubs/admin.json", URL: "/users", Method: "POST",
			HeaderMatch: map[string]string{"X-Tenant": "acme"}, BodyMatch: []byte(`{"role":"admin"}`), Status: 201},
		{Mock: "users", FilePath: "/m/http/users/stubs/list.json", URL: "/users", Method: "GET", Status: 200},
		{Mock: "users", FilePath: "/m/http/users/stubs/by_id.json", URL: "/users/[0-9]+", Method: "GET", Status: 200},
	}
}

func serveRouter(t *testing.T, method, path, body string, headers map[string]string) (*httptest.ResponseRecorder, noMatch) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	NewRouter(noMatchRoutes(), fakeUI).ServeHTTP(rec, req)
	var resp noMatch
	if rec.Code == http.StatusNotFound {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("404 body is not JSON: %v\n%s", err, rec.Body)
		}
	}
	return rec, resp
}

func TestNoMatch_ExplainsClosestStub(t *testing.T) {
	rec, resp := serveRouter(t, "POST", "/users", `{"role":"user"}`, map[string]string{"X-Tenant": "other"})

	if rec.Code != 404 || rec.Header().Get("X-Protomock-No-Match") != "true" {
		t.Fatalf("status = %d, header = %q", rec.Code, rec.Header().Get("X-Protomock-No-Match"))
	}
	if resp.Error != "protomock: no stub matched POST /users" {
		t.Errorf("error = %q", resp.Error)
	}
	if len(resp.Closest) == 0 || resp.Closest[0].Stub != "users/stubs/admin.json" {
		t.Fatalf("closest = %+v, want admin.json first (fewest failed rules)", resp.Closest)
	}
	want := []string{`header X-Tenant: expected "acme", got "other"`, `body.role: expected "admin", got "user"`}
	if got := resp.Closest[0].Reasons; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("reasons = %q, want %q", got, want)
	}
}

func TestNoMatch_MissingHeaderAndWrongMethod(t *testing.T) {
	_, resp := serveRouter(t, "DELETE", "/users", "", nil)
	reasons := map[string][]string{}
	for _, c := range resp.Closest {
		reasons[c.Stub] = c.Reasons
	}
	if got := strings.Join(reasons["users/stubs/list.json"], "|"); got != "method: expected GET, got DELETE" {
		t.Errorf("list.json reasons = %q", got)
	}
	if got := strings.Join(reasons["users/stubs/admin.json"], "|"); !strings.Contains(got, `header X-Tenant: missing (expected "acme")`) ||
		!strings.Contains(got, "body: request has no body") {
		t.Errorf("admin.json reasons = %q", got)
	}
}

func TestNoMatch_UnknownURLSuggestsSimilar(t *testing.T) {
	_, resp := serveRouter(t, "GET", "/user", "", nil)
	if resp.Error != "protomock: no stub has URL /user" {
		t.Errorf("error = %q", resp.Error)
	}
	if len(resp.Similar) == 0 || !strings.HasPrefix(resp.Similar[0].URL, "/users") {
		t.Errorf("similar = %+v, want /users stubs first", resp.Similar)
	}
}

func TestRouter_ExactURLFallsThroughToRegex(t *testing.T) {
	routes := []models.Route{
		{URL: "/items/42", Method: "POST", Status: 201, RawJSONBody: []byte(`"exact"`)},
		{URL: "/items/[0-9]+", Method: "GET", Status: 200, RawJSONBody: []byte(`"regex"`)},
	}
	rec := httptest.NewRecorder()
	NewRouter(routes, fakeUI).ServeHTTP(rec, httptest.NewRequest("GET", "/items/42", nil))
	if rec.Code != 200 || rec.Body.String() != `"regex"` {
		t.Errorf("GET /items/42 = %d %s; the regex stub should answer when no exact stub matches", rec.Code, rec.Body)
	}
}
