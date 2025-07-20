package grpcserver

import (
	"encoding/json"
	"strings"

	"github.com/avijeet7/protomock/internal/models"
)

func normalizeRoutes(routes []models.Route) map[string]models.Route {
	m := make(map[string]models.Route)
	for _, r := range routes {
		key := normalizeGRPCMethod(r.URL)
		m[key] = r
	}
	return m
}

func normalizeGRPCMethod(url string) string {
	if !strings.HasPrefix(url, "/") {
		return "/" + url
	}
	return url
}

// partialJSONMatch checks whether all fields in `expected` are present in `actual`
func partialJSONMatch(expectedRaw, actualRaw []byte) bool {
	var expected, actual map[string]interface{}
	_ = json.Unmarshal(expectedRaw, &expected)
	_ = json.Unmarshal(actualRaw, &actual)

	for k, v := range expected {
		if actual[k] != v {
			return false
		}
	}
	return true
}
