package app

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// startupBanner is the "where can I open it" message printed once the servers listen.
//
// Inside a container ProtoMock can't see which host ports Docker maps to its ports
// (e.g. "8888:8085"), so the listen addresses are only right for the host if the
// mapping is the same port. publicURL (PROTOMOCK_PUBLIC_URL) overrides the printed URL.
func startupBanner(httpAddr, grpcAddr net.Addr, publicURL string, container bool) []string {
	base := publicURL
	if base == "" {
		base = "http://" + browsableAddr(httpAddr.String())
	}
	lines := []string{
		fmt.Sprintf("🌐 ProtoMock UI:  %s/protomock-ui", base),
		fmt.Sprintf("   HTTP mocks:    %s", base),
		fmt.Sprintf("   gRPC mocks:    %s", browsableAddr(grpcAddr.String())),
	}
	if container && publicURL == "" {
		_, httpPort, _ := net.SplitHostPort(httpAddr.String())
		_, grpcPort, _ := net.SplitHostPort(grpcAddr.String())
		lines = append(lines, fmt.Sprintf(
			"ℹ️ Running in a container: these are container ports (HTTP %s, gRPC %s). On your machine, use the host port mapped to them "+
				"in docker-compose (e.g. \"8888:%s\" → http://localhost:8888/protomock-ui), or set PROTOMOCK_PUBLIC_URL to print the exact URL.",
			httpPort, grpcPort, httpPort))
	}
	return lines
}

// browsableAddr turns a listen address into one a browser or client can use:
// listening on all interfaces ("[::]:8085", ":8085") becomes "localhost:8085".
func browsableAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "localhost"
	}
	return net.JoinHostPort(host, port)
}

// dialAddr is like browsableAddr but uses the loopback IP, for connecting from this process.
func dialAddr(addr string) string {
	return strings.Replace(browsableAddr(addr), "localhost:", "127.0.0.1:", 1)
}

// inContainer reports whether ProtoMock runs in a Docker/Podman container or Kubernetes pod.
func inContainer() bool {
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}
