package auth

import (
	"errors"
	"testing"
)

func TestIdentityFromClaims(t *testing.T) {
	testCases := []struct {
		name    string
		claims  oidcIdentityClaims
		want    Identity
		wantErr error
	}{
		{
			name:   "provider identity",
			claims: oidcIdentityClaims{Subject: "service-account-provider-a", ProviderID: "provider-a"},
			want:   Identity{Subject: "service-account-provider-a", ProviderID: "provider-a"},
		},
		{
			name:   "internal identity",
			claims: oidcIdentityClaims{Subject: "service-account-internal", Internal: true},
			want:   Identity{Subject: "service-account-internal", Internal: true},
		},
		{name: "missing subject", claims: oidcIdentityClaims{ProviderID: "provider-a"}, wantErr: ErrInvalidTokenClaims},
		{name: "missing authorization", claims: oidcIdentityClaims{Subject: "service-account"}, wantErr: ErrInvalidTokenClaims},
		{name: "ambiguous identity", claims: oidcIdentityClaims{Subject: "service-account", ProviderID: "provider-a", Internal: true}, wantErr: ErrInvalidTokenClaims},
		{name: "provider with surrounding whitespace", claims: oidcIdentityClaims{Subject: "service-account", ProviderID: " provider-a "}, wantErr: ErrInvalidTokenClaims},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			identity, err := identityFromClaims(testCase.claims)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("identityFromClaims() error = %v, want %v", err, testCase.wantErr)
			}
			if identity != testCase.want {
				t.Fatalf("identityFromClaims() = %#v, want %#v", identity, testCase.want)
			}
		})
	}
}
