package app

import (
	"testing"
	"time"

	"github.com/avijeet7/protomock/internal/config"
)

func TestWatchStopsWhenAsked(t *testing.T) {
	dir := t.TempDir()
	a := &App{cfg: config.Config{MocksDir: dir}, stop: make(chan struct{})}
	a.fingerprint = fingerprintDir(dir) // unchanged folder: the watcher never needs to reload

	done := make(chan struct{})
	go func() {
		a.watch(5 * time.Millisecond)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // let it tick a few times
	a.stopWatching()
	a.stopWatching() // stopping twice must be safe

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watch goroutine is still running after stopWatching")
	}
}
