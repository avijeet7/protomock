//go:build integration

package integration

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/avijeet7/protomock/internal/grpcserver"
	"github.com/avijeet7/protomock/internal/httpserver"
)

// occupyPort listens on a random local port and returns its address,
// so a server started on the same address must fail.
func occupyPort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lis.Close() })
	return lis.Addr().String()
}

// expectStartError runs start in the background and requires it to return an error quickly.
func expectStartError(t *testing.T, start func() error) {
	t.Helper()
	errCh := make(chan error, 1)
	go func() { errCh <- start() }()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("server returned nil error, want address-in-use error")
		}
		t.Logf("got expected error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("server did not return an error (still running?)")
	}
}

func TestStartup_HTTPPortInUseReturnsError(t *testing.T) {
	addr := occupyPort(t)
	expectStartError(t, func() error {
		return httpserver.NewServer(nil, http.NotFoundHandler()).ListenAndServe(addr)
	})
}

func TestStartup_GRPCPortInUseReturnsError(t *testing.T) {
	addr := occupyPort(t)
	expectStartError(t, func() error {
		return grpcserver.NewServer(nil).ListenAndServe(addr)
	})
}
