//go:build integration

package integration

import (
	"net"
	"strings"
	"testing"

	"github.com/avijeet7/protomock/internal/app"
	"github.com/avijeet7/protomock/internal/config"
)

const brokenStub = `{"request":{"url":"/no-method"}}`

func TestHealth_OK(t *testing.T) {
	mocks := t.TempDir()
	writeFiles(t, mocks, editorFixture)
	_, base, _, cfg := startAppWith(t, config.Config{MocksDir: mocks})

	code, body := get(t, base+"/protomock-ui/api/health")
	if code != 200 || !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("health = %d %s", code, body)
	}
	// Not strict: problems (editorFixture has a broken stub) don't make it unhealthy.
	if !strings.Contains(body, `"problems":1`) {
		t.Errorf("health should report the problem count: %s", body)
	}
	if err := app.HealthCheck(cfg); err != nil {
		t.Errorf("HealthCheck: %v", err)
	}
}

func TestHealth_NothingListening(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close() // now nothing listens: connections are refused immediately
	if err := app.HealthCheck(config.Config{HTTPAddr: addr, GRPCAddr: addr}); err == nil {
		t.Error("HealthCheck should fail when nothing answers HTTP")
	}
}

func TestStrict_StartupFailsOnBrokenMocks(t *testing.T) {
	mocks := t.TempDir()
	writeFiles(t, mocks, map[string]string{
		"http/users/stubs/ok.json":     `{"request":{"method":"GET","url":"/ok"}}`,
		"http/users/stubs/broken.json": brokenStub,
	})
	_, err := app.New(config.Config{MocksDir: mocks, Strict: true})
	if err == nil || !strings.Contains(err.Error(), "strict mode: 1 mock file(s) failed to load") ||
		!strings.Contains(err.Error(), "broken.json: request.method is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestStrict_HealthUnhealthyAfterBrokenReload(t *testing.T) {
	mocks := t.TempDir()
	writeFiles(t, mocks, map[string]string{"http/users/stubs/ok.json": `{"request":{"method":"GET","url":"/ok"}}`})
	a, base, _, cfg := startAppWith(t, config.Config{MocksDir: mocks, Strict: true})

	if err := app.HealthCheck(cfg); err != nil {
		t.Fatalf("healthy at start: %v", err)
	}
	writeFiles(t, mocks, map[string]string{"http/users/stubs/broken.json": brokenStub})
	if _, err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	if code, body := get(t, base+"/protomock-ui/api/health"); code != 503 || !strings.Contains(body, `"status":"unhealthy"`) {
		t.Errorf("health after broken reload = %d %s", code, body)
	}
	if err := app.HealthCheck(cfg); err == nil || !strings.Contains(err.Error(), "unhealthy (503)") {
		t.Errorf("HealthCheck = %v, want unhealthy", err)
	}
}
