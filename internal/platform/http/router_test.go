package httpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRouterRegistersOpenWalletRoute(t *testing.T) {
	handler, err := NewOpenWalletHandler(
		&openWalletExecutorStub{},
		&sequenceIDGenerator{},
		fixedClock{now: time.Now()},
	)
	if err != nil {
		t.Fatalf("NewOpenWalletHandler() unexpected error: %v", err)
	}
	healthHandler, err := NewHealthHandler(&databaseHealthCheckerStub{})
	if err != nil {
		t.Fatalf("NewHealthHandler() unexpected error: %v", err)
	}
	router := NewRouter(handler, healthHandler)

	request := httptest.NewRequest(http.MethodGet, "/wallets", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /wallets status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestRouterRegistersHealthRoutes(t *testing.T) {
	openWalletHandler, err := NewOpenWalletHandler(
		&openWalletExecutorStub{},
		&sequenceIDGenerator{},
		fixedClock{now: time.Now()},
	)
	if err != nil {
		t.Fatalf("NewOpenWalletHandler() unexpected error: %v", err)
	}
	healthHandler, err := NewHealthHandler(&databaseHealthCheckerStub{})
	if err != nil {
		t.Fatalf("NewHealthHandler() unexpected error: %v", err)
	}
	router := NewRouter(openWalletHandler, healthHandler)

	for _, path := range []string{"/health/live", "/health/ready"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", path, response.Code, http.StatusOK)
		}
	}
}
