package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

func TestKeycloakClientCredentialsIdentities(t *testing.T) {
	issuerURL := os.Getenv("TEST_OIDC_ISSUER_URL")
	if issuerURL == "" {
		t.Skip("TEST_OIDC_ISSUER_URL is not configured")
	}

	verifier, err := platformauth.NewOIDCTokenVerifier(config.OIDCConfig{
		IssuerURL: issuerURL,
		Audience:  "jungle-api",
	})
	if err != nil {
		t.Fatalf("NewOIDCTokenVerifier() unexpected error: %v", err)
	}

	testCases := []struct {
		clientID     string
		clientSecret string
		providerID   string
		internal     bool
	}{
		{clientID: "provider-a", clientSecret: "provider-a-secret", providerID: "provider-a"},
		{clientID: "provider-b", clientSecret: "provider-b-secret", providerID: "provider-b"},
		{clientID: "internal-service", clientSecret: "internal-service-secret", internal: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.clientID, func(t *testing.T) {
			token := requestClientCredentialsToken(
				t,
				issuerURL,
				testCase.clientID,
				testCase.clientSecret,
			)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			identity, err := verifier.Verify(ctx, token)
			if err != nil {
				t.Fatalf("Verify() unexpected error: %v", err)
			}
			if identity.Subject == "" || identity.ProviderID != testCase.providerID ||
				identity.Internal != testCase.internal {
				t.Fatalf("verified identity = %#v", identity)
			}
		})
	}
}

func requestClientCredentialsToken(
	t *testing.T,
	issuerURL string,
	clientID string,
	clientSecret string,
) string {
	t.Helper()
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	request, err := http.NewRequest(
		http.MethodPost,
		strings.TrimSuffix(issuerURL, "/")+"/protocol/openid-connect/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		t.Fatalf("create token request: %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request token: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("token endpoint status = %d, want 200", response.StatusCode)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if payload.AccessToken == "" {
		t.Fatalf("empty access token for client %s", clientID)
	}
	return payload.AccessToken
}
