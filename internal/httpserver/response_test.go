package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/avijeet7/protomock/internal/models"
)

func TestWriteResponse_Headers(t *testing.T) {
	route := models.Route{
		Status:      http.StatusAccepted,
		RawJSONBody: []byte(`<ok/>`),
		Headers:     map[string]string{"Content-Type": "application/xml", "X-Request-Id": "abc"},
	}
	rec := httptest.NewRecorder()
	writeResponse(rec, httptest.NewRequest(http.MethodGet, "/", nil), route)

	res := rec.Result()
	if res.StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want 202", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "application/xml" {
		t.Errorf("Content-Type = %q, want stub override application/xml", got)
	}
	if got := res.Header.Get("X-Request-Id"); got != "abc" {
		t.Errorf("X-Request-Id = %q, want abc", got)
	}
}

func TestWriteResponse_Delay(t *testing.T) {
	route := models.Route{Status: 200, RawJSONBody: []byte(`{}`), Delay: 80 * time.Millisecond}

	t.Run("waits before responding", func(t *testing.T) {
		start := time.Now()
		rec := httptest.NewRecorder()
		writeResponse(rec, httptest.NewRequest(http.MethodGet, "/", nil), route)
		if elapsed := time.Since(start); elapsed < route.Delay {
			t.Errorf("responded after %v, want at least %v", elapsed, route.Delay)
		}
		if rec.Code != 200 {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("stops waiting when the client gives up", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		start := time.Now()
		rec := httptest.NewRecorder()
		writeResponse(rec, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx), route)
		if elapsed := time.Since(start); elapsed >= route.Delay {
			t.Errorf("kept waiting %v after the client gave up", elapsed)
		}
		if rec.Body.Len() != 0 {
			t.Error("no body should be written after the client gave up")
		}
	})
}
