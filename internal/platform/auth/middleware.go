package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

var ErrTokenVerifierRequired = errors.New("token verifier is required")

type Identity struct {
	Subject    string
	ProviderID string
	Internal   bool
}

type TokenVerifier interface {
	Verify(context.Context, string) (Identity, error)
}

type Middleware struct {
	verifier TokenVerifier
}

func NewMiddleware(verifier TokenVerifier) (*Middleware, error) {
	if verifier == nil {
		return nil, ErrTokenVerifierRequired
	}
	return &Middleware{verifier: verifier}, nil
}

func (middleware *Middleware) RequireProvider(next http.Handler) http.Handler {
	return middleware.authenticate(next, func(identity Identity) bool {
		return identity.ProviderID != ""
	})
}

func (middleware *Middleware) RequireInternal(next http.Handler) http.Handler {
	return middleware.authenticate(next, func(identity Identity) bool {
		return identity.Internal
	})
}

func (middleware *Middleware) authenticate(
	next http.Handler,
	authorized func(Identity) bool,
) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		rawToken, ok := bearerToken(request.Header.Get("Authorization"))
		if !ok {
			writeAuthenticationError(response, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		identity, err := middleware.verifier.Verify(request.Context(), rawToken)
		if err != nil || identity.Subject == "" {
			writeAuthenticationError(response, http.StatusUnauthorized, "UNAUTHORIZED", "invalid access token")
			return
		}
		if !authorized(identity) {
			writeAuthenticationError(response, http.StatusForbidden, "FORBIDDEN", "access denied")
			return
		}

		next.ServeHTTP(response, request.WithContext(
			ContextWithIdentity(request.Context(), identity),
		))
	})
}

func bearerToken(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

type identityContextKey struct{}

func ContextWithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok
}

func writeAuthenticationError(
	response http.ResponseWriter,
	status int,
	code string,
	message string,
) {
	response.Header().Set("Content-Type", "application/json")
	if status == http.StatusUnauthorized {
		response.Header().Set("WWW-Authenticate", "Bearer")
	}
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
