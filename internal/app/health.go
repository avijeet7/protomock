package app

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/avijeet7/protomock/internal/config"
)

const healthTimeout = 3 * time.Second

// HealthCheck checks a running ProtoMock (used by `protomock healthcheck` and the Docker
// HEALTHCHECK): the HTTP health endpoint must answer 200 and the gRPC port must accept
// connections. It uses the same config as the server, so it checks the right ports.
func HealthCheck(cfg config.Config) error {
	url := "http://" + dialAddr(cfg.HTTPAddr) + "/protomock-ui/api/health"
	client := &http.Client{Timeout: healthTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	conn, err := net.DialTimeout("tcp", dialAddr(cfg.GRPCAddr), healthTimeout)
	if err != nil {
		return fmt.Errorf("gRPC server: %w", err)
	}
	return conn.Close()
}
