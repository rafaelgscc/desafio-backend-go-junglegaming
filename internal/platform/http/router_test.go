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
	router := NewRouter(handler, healthHandler, wagerHandler, newRouterAuthMiddleware(t))

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
	router := NewRouter(openWalletHandler, healthHandler, wagerHandler, newRouterAuthMiddleware(t))

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
	router := NewRouter(openWalletHandler, healthHandler, wagerHandler, newRouterAuthMiddleware(t))

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
	router := NewRouter(openWalletHandler, healthHandler, wagerHandler, newRouterAuthMiddleware(t))

	testCases := []struct {
		name          string
		path          string
		authorization string
		wantStatus    int
	}{
		{name: "wallet without token", path: "/wallets", wantStatus: http.StatusUnauthorized},
		{name: "wallet with provider token", path: "/wallets", authorization: "Bearer provider", wantStatus: http.StatusForbidden},
		{name: "wager without token", path: "/wagering/transactions", wantStatus: http.StatusUnauthorized},
		{name: "wager with internal token", path: "/wagering/transactions", authorization: "Bearer internal", wantStatus: http.StatusForbidden},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, testCase.path, nil)
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
