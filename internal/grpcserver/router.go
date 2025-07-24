package grpcserver

import (
	"strings"

	"github.com/avijeet7/protomock/internal/models"
)

// normalizeGRPCMethod ensures route URL starts with a leading slash
func normalizeGRPCMethod(url string) string {
	if !strings.HasPrefix(url, "/") {
		return "/" + url
	}
	return url
}

// groupRoutesByMethod groups multiple stubs under the same gRPC method
func groupRoutesByMethod(routes []models.Route) map[string][]models.Route {
	m := make(map[string][]models.Route)
	for _, r := range routes {
		key := normalizeGRPCMethod(r.URL)
		m[key] = append(m[key], r)
	}
	return m
}
