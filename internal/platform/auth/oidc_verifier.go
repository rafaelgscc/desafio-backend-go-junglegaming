package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

const oidcDiscoveryTimeout = 10 * time.Second

var ErrInvalidTokenClaims = errors.New("invalid access token claims")

type OIDCTokenVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewOIDCTokenVerifier(configuration config.OIDCConfig) (*OIDCTokenVerifier, error) {
	ctx, cancel := context.WithTimeout(context.Background(), oidcDiscoveryTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, configuration.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	return &OIDCTokenVerifier{
		verifier: provider.Verifier(&oidc.Config{ClientID: configuration.Audience}),
	}, nil
}

func (verifier *OIDCTokenVerifier) Verify(
	ctx context.Context,
	rawToken string,
) (Identity, error) {
	token, err := verifier.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verify access token: %w", err)
	}
	var claims oidcIdentityClaims
	if err := token.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("decode access token claims: %w", err)
	}
	return identityFromClaims(claims)
}

type oidcIdentityClaims struct {
	Subject    string `json:"sub"`
	ProviderID string `json:"provider_id"`
	Internal   bool   `json:"internal"`
}

func identityFromClaims(claims oidcIdentityClaims) (Identity, error) {
	if claims.Subject == "" || claims.Subject != strings.TrimSpace(claims.Subject) {
		return Identity{}, ErrInvalidTokenClaims
	}
	hasProvider := claims.ProviderID != ""
	if hasProvider && claims.ProviderID != strings.TrimSpace(claims.ProviderID) {
		return Identity{}, ErrInvalidTokenClaims
	}
	if hasProvider == claims.Internal {
		return Identity{}, ErrInvalidTokenClaims
	}
	return Identity{
		Subject: claims.Subject, ProviderID: claims.ProviderID, Internal: claims.Internal,
	}, nil
}
