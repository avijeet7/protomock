//go:build integration

package integration

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/avijeet7/protomock/internal/app"
	"github.com/avijeet7/protomock/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// startApp runs the whole app (loader, HTTP + gRPC servers, watcher) on random local ports.
func startApp(t *testing.T, mocksDir string, reloadInterval time.Duration) (a *app.App, httpURL string, grpcConn *grpc.ClientConn) {
	t.Helper()
	a, httpURL, grpcConn, _ = startAppWith(t, config.Config{MocksDir: mocksDir, ReloadInterval: reloadInterval})
	return a, httpURL, grpcConn
}

// startAppWith runs the app with cfg on random ports; cfg's addresses are set to the real ones.
func startAppWith(t *testing.T, cfg config.Config) (a *app.App, httpURL string, grpcConn *grpc.ClientConn, actual config.Config) {
	t.Helper()
	a, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	httpLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = a.Serve(httpLis, grpcLis) }()
	t.Cleanup(func() { _ = httpLis.Close(); _ = grpcLis.Close() })
	cfg.HTTPAddr, cfg.GRPCAddr = httpLis.Addr().String(), grpcLis.Addr().String()

	conn, err := grpc.NewClient(grpcLis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return a, "http://" + httpLis.Addr().String(), conn, cfg
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(body))
}

// eventually retries check until it passes or the timeout expires.
func eventually(t *testing.T, timeout time.Duration, check func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func TestReload_PicksUpFileChangesWithoutRestart(t *testing.T) {
	mocks := t.TempDir()
	writeFiles(t, mocks, map[string]string{
		"http/users/stubs/list.json": `{"request":{"method":"GET","url":"/users"},"response":{"body":["bob"]}}`,
	})
	_, base, conn := startApp(t, mocks, 50*time.Millisecond)

	if code, body := get(t, base+"/users"); code != 200 || body != `["bob"]` {
		t.Fatalf("before change: %d %s", code, body)
	}

	// Edit a stub, add an HTTP stub and a whole gRPC mock — no restart.
	writeFiles(t, mocks, map[string]string{
		"http/users/stubs/list.json":    `{"request":{"method":"GET","url":"/users"},"response":{"body":["bob","alice"]}}`,
		"http/users/stubs/created.json": `{"request":{"method":"POST","url":"/users"},"response":{"status":201,"body":{"id":2}}}`,
		"grpc/greeter/greeter.proto":    greeterProto,
		"grpc/greeter/stubs/hello.json": greeterStub("bob", "hi from reload"),
	})

	if !eventually(t, 3*time.Second, func() bool {
		_, body := get(t, base+"/users")
		return body == `["bob","alice"]`
	}) {
		t.Fatal("edited stub was not reloaded")
	}
	resp, err := http.Post(base+"/users", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("new stub: status = %d, want 201", resp.StatusCode)
	}
	if got, err := callHello(t, conn, "bob"); err != nil || got != "hi from reload" {
		t.Errorf("new gRPC mock: reply = %q, err = %v", got, err)
	}
}

func TestReload_Manual(t *testing.T) {
	mocks := t.TempDir()
	a, base, _ := startApp(t, mocks, 0) // watcher off

	writeFiles(t, mocks, map[string]string{
		"http/ping/stubs/ping.json": `{"request":{"method":"GET","url":"/ping"},"response":{"body":"pong"}}`,
	})
	if code, _ := get(t, base+"/ping"); code != 404 {
		t.Fatalf("with the watcher off, the new stub should not be served before Reload; got %d", code)
	}
	set, err := a.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if len(set.HTTP) != 1 {
		t.Fatalf("Reload loaded %d HTTP routes, want 1", len(set.HTTP))
	}
	if code, body := get(t, base+"/ping"); code != 200 || body != `"pong"` {
		t.Errorf("after Reload: %d %s", code, body)
	}
}
