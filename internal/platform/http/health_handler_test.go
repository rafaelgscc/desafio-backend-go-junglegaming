package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	t.Run("reports the process as live without checking dependencies", func(t *testing.T) {
		checker := &databaseHealthCheckerStub{}
		handler, err := NewHealthHandler(checker)
		if err != nil {
			t.Fatalf("NewHealthHandler() unexpected error: %v", err)
		}

		request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		response := httptest.NewRecorder()

		handler.Live(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
		}
		if checker.calls != 0 {
			t.Fatalf("database checks = %d, want 0", checker.calls)
		}
		if got := response.Body.String(); got != "{\"status\":\"ok\"}\n" {
			t.Fatalf("body = %q, want healthy response", got)
		}
	})

	t.Run("reports ready when the database is reachable", func(t *testing.T) {
		checker := &databaseHealthCheckerStub{}
		handler, err := NewHealthHandler(checker)
		if err != nil {
			t.Fatalf("NewHealthHandler() unexpected error: %v", err)
		}

		request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		response := httptest.NewRecorder()

		handler.Ready(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
		}
		if checker.calls != 1 {
			t.Fatalf("database checks = %d, want 1", checker.calls)
		}
	})

	t.Run("reports unavailable when the database cannot be reached", func(t *testing.T) {
		checker := &databaseHealthCheckerStub{err: errors.New("database unavailable")}
		handler, err := NewHealthHandler(checker)
		if err != nil {
			t.Fatalf("NewHealthHandler() unexpected error: %v", err)
		}

		request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		response := httptest.NewRecorder()

		handler.Ready(response, request)

		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusServiceUnavailable, response.Body)
		}
		if got := response.Body.String(); got != "{\"status\":\"unavailable\"}\n" {
			t.Fatalf("body = %q, want unavailable response", got)
		}
	})
}

type databaseHealthCheckerStub struct {
	err   error
	calls int
}

func (stub *databaseHealthCheckerStub) Ping(context.Context) error {
	stub.calls++
	return stub.err
}
