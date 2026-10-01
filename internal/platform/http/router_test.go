package httpadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"
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
	router := NewRouter(handler, healthHandler, wagerHandler, &WageringQueryHandler{}, newRouterAuthMiddleware(t))

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
	router := NewRouter(openWalletHandler, healthHandler, wagerHandler, &WageringQueryHandler{}, newRouterAuthMiddleware(t))

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
	router := NewRouter(openWalletHandler, healthHandler, wagerHandler, &WageringQueryHandler{}, newRouterAuthMiddleware(t))

	request := httptest.NewRequest(http.MethodGet, "/wagering/transactions", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /wagering/transactions status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestRouterProtectsBusinessRoutes(t *testing.T) {
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
	wagerHandler := newWagerTransactionHandlerForTest(t, &executeWagerTransactionStub{}, time.Now())
	router := NewRouter(openWalletHandler, healthHandler, wagerHandler, &WageringQueryHandler{}, newRouterAuthMiddleware(t))

	testCases := []struct {
		name          string
		method        string
		path          string
		authorization string
		wantStatus    int
	}{
		{name: "wallet without token", method: http.MethodPost, path: "/wallets", wantStatus: http.StatusUnauthorized},
		{name: "wallet with provider token", method: http.MethodPost, path: "/wallets", authorization: "Bearer provider", wantStatus: http.StatusForbidden},
		{name: "wallet query without token", method: http.MethodGet, path: "/wallets/wallet-1", wantStatus: http.StatusUnauthorized},
		{name: "ledger with provider token", method: http.MethodGet, path: "/wallets/wallet-1/ledger", authorization: "Bearer provider", wantStatus: http.StatusForbidden},
		{name: "reconciliation without token", method: http.MethodPost, path: "/wallets/wallet-1/reconciliation", wantStatus: http.StatusUnauthorized},
		{name: "wager without token", method: http.MethodPost, path: "/wagering/transactions", wantStatus: http.StatusUnauthorized},
		{name: "wager with internal token", method: http.MethodPost, path: "/wagering/transactions", authorization: "Bearer internal", wantStatus: http.StatusForbidden},
		{name: "wager query without token", method: http.MethodGet, path: "/wagering/transactions/transaction-1", wantStatus: http.StatusUnauthorized},
		{name: "external wager query with internal token", method: http.MethodGet, path: "/providers/provider-a/wagering/transactions/external-1", authorization: "Bearer internal", wantStatus: http.StatusForbidden},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			request.Header.Set("Authorization", testCase.authorization)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, testCase.wantStatus, response.Body)
			}
		})
	}
}

type routerTokenVerifier struct{}

func (routerTokenVerifier) Verify(_ context.Context, token string) (platformauth.Identity, error) {
	if token == "internal" {
		return platformauth.Identity{Subject: "internal-service", Internal: true}, nil
	}
	return platformauth.Identity{Subject: "provider-service", ProviderID: "provider-a"}, nil
}

func newRouterAuthMiddleware(t *testing.T) *platformauth.Middleware {
	t.Helper()
	middleware, err := platformauth.NewMiddleware(routerTokenVerifier{})
	if err != nil {
		t.Fatalf("NewMiddleware() unexpected error: %v", err)
	}
	return middleware
}
