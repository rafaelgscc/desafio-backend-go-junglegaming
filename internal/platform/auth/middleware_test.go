package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareAuthenticationAndAuthorization(t *testing.T) {
	verifier := tokenVerifierStub{
		identities: map[string]Identity{
			"provider-token": {Subject: "service-account-provider-a", ProviderID: "provider-a"},
			"internal-token": {Subject: "service-account-internal", Internal: true},
		},
	}
	middleware, err := NewMiddleware(verifier)
	if err != nil {
		t.Fatalf("NewMiddleware() unexpected error: %v", err)
	}

	providerHandler := middleware.RequireProvider(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		identity, ok := IdentityFromContext(request.Context())
		if !ok || identity.ProviderID != "provider-a" {
			t.Fatalf("provider identity missing from context: %#v", identity)
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	internalHandler := middleware.RequireInternal(http.HandlerFunc(func(
		response http.ResponseWriter,
		_ *http.Request,
	) {
		response.WriteHeader(http.StatusNoContent)
	}))

	testCases := []struct {
		name          string
		handler       http.Handler
		authorization string
		wantStatus    int
	}{
		{name: "missing token", handler: providerHandler, wantStatus: http.StatusUnauthorized},
		{name: "invalid token", handler: providerHandler, authorization: "Bearer invalid", wantStatus: http.StatusUnauthorized},
		{name: "malformed authorization", handler: providerHandler, authorization: "Basic abc", wantStatus: http.StatusUnauthorized},
		{name: "provider accepted", handler: providerHandler, authorization: "Bearer provider-token", wantStatus: http.StatusNoContent},
		{name: "internal denied provider route", handler: providerHandler, authorization: "Bearer internal-token", wantStatus: http.StatusForbidden},
		{name: "provider denied internal route", handler: internalHandler, authorization: "Bearer provider-token", wantStatus: http.StatusForbidden},
		{name: "internal accepted", handler: internalHandler, authorization: "Bearer internal-token", wantStatus: http.StatusNoContent},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", nil)
			request.Header.Set("Authorization", testCase.authorization)
			response := httptest.NewRecorder()

			testCase.handler.ServeHTTP(response, request)

			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, testCase.wantStatus, response.Body)
			}
			if testCase.wantStatus == http.StatusUnauthorized &&
				response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("WWW-Authenticate = %q, want Bearer", response.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

type tokenVerifierStub struct {
	identities map[string]Identity
}

func (stub tokenVerifierStub) Verify(_ context.Context, rawToken string) (Identity, error) {
	identity, ok := stub.identities[rawToken]
	if !ok {
		return Identity{}, errors.New("invalid token")
	}
	return identity, nil
}
