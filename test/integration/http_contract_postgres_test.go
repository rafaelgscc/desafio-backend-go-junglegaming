package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"
	httpadapter "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/http"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

func TestHTTPContractWithPostgresAndProviderIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool := openConcurrencyTestPools(t, ctx, "http_contract_test", 1)[0]
	unitOfWork := mustWageringUnitOfWork(t, pool)

	openWallet, err := application.NewOpenWalletUseCase(unitOfWork)
	if err != nil {
		t.Fatal(err)
	}
	wagerExecutor := newPostgresWagerExecutor(t, pool)
	queryRepository, err := platformpostgres.NewWageringQueryRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	queryService, err := application.NewWageringQueryService(queryRepository)
	if err != nil {
		t.Fatal(err)
	}
	idGenerator := &integrationSequenceIDGenerator{}
	clock := integrationFixedClock{now: time.Date(2026, time.October, 2, 15, 0, 0, 0, time.UTC)}
	openWalletHandler, err := httpadapter.NewOpenWalletHandler(openWallet, idGenerator, clock)
	if err != nil {
		t.Fatal(err)
	}
	wagerHandler, err := httpadapter.NewWagerTransactionHandler(wagerExecutor, idGenerator, clock)
	if err != nil {
		t.Fatal(err)
	}
	queryHandler, err := httpadapter.NewWageringQueryHandler(queryService)
	if err != nil {
		t.Fatal(err)
	}
	healthHandler, err := httpadapter.NewHealthHandler(pool)
	if err != nil {
		t.Fatal(err)
	}
	authMiddleware, err := platformauth.NewMiddleware(httpContractTokenVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	router := httpadapter.NewRouter(
		openWalletHandler, healthHandler, wagerHandler, queryHandler, authMiddleware,
	)

	for _, path := range []string{"/health/live", "/health/ready"} {
		response := performHTTPContractRequest(router, http.MethodGet, path, "", "")
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%s", path, response.Code, response.Body)
		}
	}

	opened := performHTTPContractRequest(router, http.MethodPost, "/wallets", "internal", `{
		"playerId":"player-1",
		"initialBalance":{"amount":"100.00","currency":"BRL"}
	}`)
	if opened.Code != http.StatusCreated {
		t.Fatalf("POST /wallets status=%d body=%s", opened.Code, opened.Body)
	}
	var walletBody struct {
		ID      string       `json:"id"`
		Balance domain.Money `json:"balance"`
		Version int64        `json:"version"`
	}
	decodeHTTPContractResponse(t, opened, &walletBody)
	if walletBody.ID == "" || walletBody.Balance.Amount() != "100.00" || walletBody.Version != 1 {
		t.Fatalf("wallet response = %#v", walletBody)
	}

	walletPath := "/wallets/" + walletBody.ID
	gotWallet := performHTTPContractRequest(router, http.MethodGet, walletPath, "internal", "")
	if gotWallet.Code != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%s", walletPath, gotWallet.Code, gotWallet.Body)
	}
	providerWallet := performHTTPContractRequest(router, http.MethodGet, walletPath, "provider-a", "")
	if providerWallet.Code != http.StatusForbidden {
		t.Fatalf("provider wallet query status=%d, want 403", providerWallet.Code)
	}

	wagerBody := fmt.Sprintf(`{
		"providerId":"provider-a",
		"externalTransactionId":"external-bet-1",
		"playerId":"player-1",
		"walletId":%q,
		"roundId":"round-1",
		"gameId":"game-1",
		"kind":"BET",
		"money":{"amount":"25.00","currency":"BRL"}
	}`, walletBody.ID)
	wager := performHTTPContractRequest(
		router, http.MethodPost, "/wagering/transactions", "provider-a", wagerBody,
	)
	if wager.Code != http.StatusBadRequest {
		// The first call intentionally demonstrates that the idempotency header is mandatory.
		t.Fatalf("wager without idempotency key status=%d body=%s", wager.Code, wager.Body)
	}
	if code := httpContractErrorCode(t, wager); code != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("wager error code=%q", code)
	}

	wager = performHTTPContractWagerRequest(router, wagerBody, "provider-a", "provider-a:external-bet-1")
	if wager.Code != http.StatusOK {
		t.Fatalf("POST /wagering/transactions status=%d body=%s", wager.Code, wager.Body)
	}
	var wagerResult struct {
		TransactionID    string       `json:"transactionId"`
		Status           string       `json:"status"`
		Balance          domain.Money `json:"balance"`
		IdempotentReplay bool         `json:"idempotentReplay"`
	}
	decodeHTTPContractResponse(t, wager, &wagerResult)
	if wagerResult.TransactionID == "" || wagerResult.Status != "PROCESSED" ||
		wagerResult.Balance.Amount() != "75.00" || wagerResult.IdempotentReplay {
		t.Fatalf("wager response = %#v", wagerResult)
	}

	replay := performHTTPContractWagerRequest(router, wagerBody, "provider-a", "provider-a:external-bet-1")
	var replayResult struct {
		TransactionID    string       `json:"transactionId"`
		Balance          domain.Money `json:"balance"`
		IdempotentReplay bool         `json:"idempotentReplay"`
	}
	decodeHTTPContractResponse(t, replay, &replayResult)
	if replay.Code != http.StatusOK || replayResult.TransactionID != wagerResult.TransactionID ||
		replayResult.Balance.Amount() != "75.00" || !replayResult.IdempotentReplay {
		t.Fatalf("replay status=%d response=%#v", replay.Code, replayResult)
	}

	transactionPath := "/wagering/transactions/" + wagerResult.TransactionID
	providerAQuery := performHTTPContractRequest(router, http.MethodGet, transactionPath, "provider-a", "")
	if providerAQuery.Code != http.StatusOK {
		t.Fatalf("provider-a transaction query status=%d body=%s", providerAQuery.Code, providerAQuery.Body)
	}
	providerBQuery := performHTTPContractRequest(router, http.MethodGet, transactionPath, "provider-b", "")
	if providerBQuery.Code != http.StatusNotFound {
		t.Fatalf("provider-b transaction query status=%d body=%s", providerBQuery.Code, providerBQuery.Body)
	}
	externalPath := "/providers/provider-a/wagering/transactions/external-bet-1"
	if response := performHTTPContractRequest(router, http.MethodGet, externalPath, "provider-a", ""); response.Code != http.StatusOK {
		t.Fatalf("external transaction query status=%d body=%s", response.Code, response.Body)
	}
	if response := performHTTPContractRequest(router, http.MethodGet, externalPath, "provider-b", ""); response.Code != http.StatusForbidden {
		t.Fatalf("cross-provider external query status=%d body=%s", response.Code, response.Body)
	}

	firstLedgerPage := performHTTPContractRequest(
		router, http.MethodGet, walletPath+"/ledger?limit=1", "internal", "",
	)
	if firstLedgerPage.Code != http.StatusOK {
		t.Fatalf("first ledger page status=%d body=%s", firstLedgerPage.Code, firstLedgerPage.Body)
	}
	var firstPage struct {
		Entries    []json.RawMessage `json:"entries"`
		NextCursor string            `json:"nextCursor"`
	}
	decodeHTTPContractResponse(t, firstLedgerPage, &firstPage)
	if len(firstPage.Entries) != 1 || firstPage.NextCursor == "" {
		t.Fatalf("first ledger page = %#v", firstPage)
	}
	secondLedgerPage := performHTTPContractRequest(
		router, http.MethodGet,
		walletPath+"/ledger?limit=1&cursor="+url.QueryEscape(firstPage.NextCursor), "internal", "",
	)
	var secondPage struct {
		Entries    []json.RawMessage `json:"entries"`
		NextCursor string            `json:"nextCursor"`
	}
	decodeHTTPContractResponse(t, secondLedgerPage, &secondPage)
	if secondLedgerPage.Code != http.StatusOK || len(secondPage.Entries) != 1 || secondPage.NextCursor != "" {
		t.Fatalf("second ledger page status=%d page=%#v", secondLedgerPage.Code, secondPage)
	}

	reconciliation := performHTTPContractRequest(
		router, http.MethodPost, walletPath+"/reconciliation", "internal", "",
	)
	if reconciliation.Code != http.StatusOK {
		t.Fatalf("reconciliation status=%d body=%s", reconciliation.Code, reconciliation.Body)
	}
	var reconciliationBody struct {
		StoredBalance     domain.Money `json:"storedBalance"`
		CalculatedBalance domain.Money `json:"calculatedBalance"`
		Difference        domain.Money `json:"difference"`
		Consistent        bool         `json:"consistent"`
		CheckedEntries    int64        `json:"checkedEntries"`
	}
	decodeHTTPContractResponse(t, reconciliation, &reconciliationBody)
	if !reconciliationBody.Consistent || reconciliationBody.StoredBalance.Amount() != "75.00" ||
		reconciliationBody.CalculatedBalance.Amount() != "75.00" ||
		reconciliationBody.Difference.Amount() != "0.00" || reconciliationBody.CheckedEntries != 2 {
		t.Fatalf("reconciliation = %#v", reconciliationBody)
	}
}

type httpContractTokenVerifier struct{}

func (httpContractTokenVerifier) Verify(
	_ context.Context,
	token string,
) (platformauth.Identity, error) {
	switch token {
	case "internal":
		return platformauth.Identity{Subject: "internal-service", Internal: true}, nil
	case "provider-a", "provider-b":
		return platformauth.Identity{Subject: "service-account-" + token, ProviderID: token}, nil
	default:
		return platformauth.Identity{}, fmt.Errorf("invalid token")
	}
}

func performHTTPContractRequest(
	handler http.Handler,
	method string,
	path string,
	token string,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func performHTTPContractWagerRequest(
	handler http.Handler,
	body string,
	token string,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPost, "/wagering/transactions", bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Idempotency-Key", idempotencyKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeHTTPContractResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	destination any,
) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode HTTP response: %v; body=%s", err, response.Body)
	}
}

func httpContractErrorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeHTTPContractResponse(t, response, &body)
	return body.Error.Code
}
