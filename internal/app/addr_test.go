package app

import (
	"net"
	"strings"
	"testing"
)

func tcpAddr(s string) net.Addr {
	a, err := net.ResolveTCPAddr("tcp", s)
	if err != nil {
		panic(err)
	}
	return a
}

func TestBrowsableAddr(t *testing.T) {
	tests := map[string]string{
		":8085":          "localhost:8085",
		"[::]:8085":      "localhost:8085",
		"0.0.0.0:9000":   "localhost:9000",
		"127.0.0.1:9000": "127.0.0.1:9000",
		"10.1.2.3:8085":  "10.1.2.3:8085",
	}
	for in, want := range tests {
		if got := browsableAddr(in); got != want {
			t.Errorf("browsableAddr(%q) = %q, want %q", in, got, want)
		}
	}
	if got := dialAddr("[::]:8086"); got != "127.0.0.1:8086" {
		t.Errorf("dialAddr = %q", got)
	}
}

func TestStartupBanner(t *testing.T) {
	httpAddr, grpcAddr := tcpAddr("[::]:9085"), tcpAddr("[::]:9086")

	t.Run("uses the actual listen ports", func(t *testing.T) {
		got := strings.Join(startupBanner(httpAddr, grpcAddr, "", false), "\n")
		for _, want := range []string{"http://localhost:9085/protomock-ui", "localhost:9086"} {
			if !strings.Contains(got, want) {
				t.Errorf("banner missing %q:\n%s", want, got)
			}
		}
		if strings.Contains(got, "container") {
			t.Errorf("no container hint expected outside a container:\n%s", got)
		}
	})

	t.Run("container without public URL explains port mapping", func(t *testing.T) {
		got := strings.Join(startupBanner(httpAddr, grpcAddr, "", true), "\n")
		if !strings.Contains(got, "Running in a container") || !strings.Contains(got, "PROTOMOCK_PUBLIC_URL") {
			t.Errorf("missing container hint:\n%s", got)
		}
	})

	t.Run("public URL wins and silences the hint", func(t *testing.T) {
		got := strings.Join(startupBanner(httpAddr, grpcAddr, "http://localhost:8888", true), "\n")
		if !strings.Contains(got, "http://localhost:8888/protomock-ui") || strings.Contains(got, "Running in a container") {
			t.Errorf("banner = \n%s", got)
		}
	})
}
