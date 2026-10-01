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
	wagerHandler := newWagerTransactionHandlerForTest(
		t,
		&executeWagerTransactionStub{},
		time.Now(),
	)
	router := NewRouter(handler, healthHandler, wagerHandler)

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
	wagerHandler := newWagerTransactionHandlerForTest(
		t,
		&executeWagerTransactionStub{},
		time.Now(),
	)
	router := NewRouter(openWalletHandler, healthHandler, wagerHandler)

	for _, path := range []string{"/health/live", "/health/ready"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", path, response.Code, http.StatusOK)
		}
	}
}

func TestRouterRegistersWagerTransactionRoute(t *testing.T) {
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
	wagerHandler := newWagerTransactionHandlerForTest(
		t,
		&executeWagerTransactionStub{},
		time.Now(),
	)
	router := NewRouter(openWalletHandler, healthHandler, wagerHandler)

	request := httptest.NewRequest(http.MethodGet, "/wagering/transactions", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /wagering/transactions status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
