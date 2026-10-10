package grpcserver

import (
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestHeadersMatch(t *testing.T) {
	// gRPC metadata keys are always lowercase on the wire.
	md := metadata.Pairs("x-test-header", "mocked1", "x-tenant", "acme")

	tests := []struct {
		name     string
		expected map[string]string
		want     bool
	}{
		{"no headers configured", nil, true},
		{"exact match", map[string]string{"x-test-header": "mocked1"}, true},
		{"stub key casing is ignored", map[string]string{"X-Test-Header": "mocked1"}, true},
		{"all headers must match", map[string]string{"x-test-header": "mocked1", "x-tenant": "acme"}, true},
		{"wrong value", map[string]string{"x-test-header": "mocked2"}, false},
		{"missing header", map[string]string{"x-other": "1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := headersMatch(tt.expected, md); got != tt.want {
				t.Errorf("headersMatch(%v) = %v, want %v", tt.expected, got, tt.want)
			}
		})
	}

	t.Run("no metadata on request", func(t *testing.T) {
		if headersMatch(map[string]string{"x-test-header": "mocked1"}, nil) {
			t.Error("expected no match when the request has no metadata")
		}
	})
}
