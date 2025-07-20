package httpserver

import (
	"encoding/json"
	"github.com/avijeet7/protomock/internal/models"
	"io"
	"net/http"
	"strings"
)

func matchMethod(r *http.Request, route models.Route) bool {
	return r.Method == route.Method
}

func matchPath(r *http.Request, route models.Route) bool {
	// Strip query params before matching
	return strings.Split(r.URL.Path, "?")[0] == route.URL
}

func matchHeaders(r *http.Request, route models.Route) bool {
	for key, expected := range route.HeaderMatch {
		if actual := r.Header.Get(key); actual != expected {
			return false
		}
	}
	return true
}

func matchBody(r *http.Request, route models.Route) bool {
	if len(route.BodyMatch) == 0 {
		return true
	}

	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	defer r.Body.Close()

	var expectedBody, actualBody map[string]interface{}
	_ = json.Unmarshal(route.BodyMatch, &expectedBody)
	_ = json.Unmarshal(reqBody, &actualBody)

	return deepEqual(expectedBody, actualBody)
}

func deepEqual(a, b map[string]interface{}) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}
