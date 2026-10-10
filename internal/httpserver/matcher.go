package httpserver

import (
	"io"
	"net/http"
	"strings"

	"github.com/avijeet7/protomock/internal/match"
	"github.com/avijeet7/protomock/internal/models"
)

func matchMethod(r *http.Request, route models.Route) bool {
	return r.Method == route.Method
}

func isRegexURL(url string) bool {
	return strings.ContainsAny(url, "*+?()[]{}")
}

func matchPath(r *http.Request, route models.Route) bool {
	if route.PathRegex != nil {
		return route.PathRegex.MatchString(r.URL.Path)
	}
	return r.URL.Path == route.URL
}

func matchHeaders(r *http.Request, route models.Route) bool {
	for key, expected := range route.HeaderMatch {
		if actual := r.Header.Get(key); actual != expected {
			return false
		}
	}
	return true
}

// readBody reads the request body once so every candidate stub can be
// matched against the same bytes (r.Body is a stream and can only be read once).
func readBody(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}
	// No Close needed: the HTTP server closes request bodies itself.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	return body
}

func matchBody(reqBody []byte, route models.Route) bool {
	if len(route.BodyMatch) == 0 {
		return true
	}
	return match.PartialJSON(route.BodyMatch, reqBody)
}
