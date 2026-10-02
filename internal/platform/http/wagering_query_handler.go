package httpadapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/observability"
)

var ErrWageringQueryExecutorRequired = errors.New("wagering query executor is required")

type WageringQueryExecutor interface {
	GetWallet(context.Context, string) (domain.Wallet, error)
	ListWalletLedger(context.Context, string, *application.LedgerCursor, int) (application.WalletLedgerPage, error)
	GetWagerTransaction(context.Context, string, string) (domain.WagerTransaction, error)
	GetWagerTransactionByExternalID(context.Context, string, string) (domain.WagerTransaction, error)
	ReconcileWallet(context.Context, string) (application.ReconciliationResult, error)
}

type WageringQueryHandler struct {
	executor WageringQueryExecutor
	metrics  *observability.Metrics
}

func NewWageringQueryHandler(
	executor WageringQueryExecutor,
	metrics *observability.Metrics,
) (*WageringQueryHandler, error) {
	if executor == nil {
		return nil, ErrWageringQueryExecutorRequired
	}
	if metrics == nil {
		return nil, observability.ErrMetricsRequired
	}
	return &WageringQueryHandler{executor: executor, metrics: metrics}, nil
}

type walletDetailsResponse struct {
	ID        string       `json:"id"`
	PlayerID  string       `json:"playerId"`
	Balance   domain.Money `json:"balance"`
	Version   int64        `json:"version"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

func (handler *WageringQueryHandler) GetWallet(response http.ResponseWriter, request *http.Request) {
	wallet, err := handler.executor.GetWallet(request.Context(), request.PathValue("walletId"))
	if err != nil {
		handler.writeQueryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, walletDetailsResponse{
		ID: wallet.ID(), PlayerID: wallet.PlayerID(), Balance: wallet.Balance(),
		Version: wallet.Version(), CreatedAt: wallet.CreatedAt(), UpdatedAt: wallet.UpdatedAt(),
	})
}

type ledgerEntryResponse struct {
	ID            string                 `json:"id"`
	WalletID      string                 `json:"walletId"`
	TransactionID string                 `json:"transactionId"`
	Direction     domain.LedgerDirection `json:"direction"`
	Money         domain.Money           `json:"money"`
	BalanceBefore domain.Money           `json:"balanceBefore"`
	BalanceAfter  domain.Money           `json:"balanceAfter"`
	CreatedAt     time.Time              `json:"createdAt"`
}

type ledgerPageResponse struct {
	Entries    []ledgerEntryResponse `json:"entries"`
	NextCursor string                `json:"nextCursor,omitempty"`
}

func (handler *WageringQueryHandler) ListWalletLedger(response http.ResponseWriter, request *http.Request) {
	limit := application.DefaultLedgerPageSize
	if rawLimit := request.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			writeError(response, http.StatusBadRequest, "INVALID_QUERY", "invalid ledger query")
			return
		}
		limit = parsed
	}
	cursor, err := decodeLedgerCursor(request.URL.Query().Get("cursor"))
	if err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_CURSOR", "invalid ledger cursor")
		return
	}
	page, err := handler.executor.ListWalletLedger(
		request.Context(), request.PathValue("walletId"), cursor, limit,
	)
	if err != nil {
		handler.writeQueryError(response, err)
		return
	}
	body := ledgerPageResponse{Entries: make([]ledgerEntryResponse, 0, len(page.Entries))}
	for _, entry := range page.Entries {
		body.Entries = append(body.Entries, ledgerEntryResponse{
			ID: entry.ID(), WalletID: entry.WalletID(), TransactionID: entry.TransactionID(),
			Direction: entry.Direction(), Money: entry.Money(), BalanceBefore: entry.BalanceBefore(),
			BalanceAfter: entry.BalanceAfter(), CreatedAt: entry.CreatedAt(),
		})
	}
	if page.NextCursor != nil {
		body.NextCursor = encodeLedgerCursor(*page.NextCursor)
	}
	writeJSON(response, http.StatusOK, body)
}

type wagerTransactionDetailsResponse struct {
	TransactionID                  string                             `json:"transactionId"`
	ExternalTransactionID          string                             `json:"externalTransactionId"`
	ProviderID                     string                             `json:"providerId"`
	WalletID                       string                             `json:"walletId"`
	PlayerID                       string                             `json:"playerId"`
	RoundID                        string                             `json:"roundId"`
	GameID                         string                             `json:"gameId"`
	Kind                           domain.WagerTransactionKind        `json:"kind"`
	Money                          domain.Money                       `json:"money"`
	Status                         domain.WagerTransactionStatus      `json:"status"`
	FailureCode                    domain.WagerTransactionFailureCode `json:"failureCode,omitempty"`
	ReferenceExternalTransactionID string                             `json:"referenceExternalTransactionId,omitempty"`
	ReferenceTransactionID         string                             `json:"referenceTransactionId,omitempty"`
	Balance                        *domain.Money                      `json:"balance,omitempty"`
	CreatedAt                      time.Time                          `json:"createdAt"`
	UpdatedAt                      time.Time                          `json:"updatedAt"`
}

func (handler *WageringQueryHandler) GetWagerTransaction(response http.ResponseWriter, request *http.Request) {
	identity, ok := platformauth.IdentityFromContext(request.Context())
	if !ok || identity.ProviderID == "" {
		writeError(response, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	transaction, err := handler.executor.GetWagerTransaction(
		request.Context(), identity.ProviderID, request.PathValue("transactionId"),
	)
	if err != nil {
		handler.writeQueryError(response, err)
		return
	}
	handler.writeWagerTransaction(response, transaction)
}

func (handler *WageringQueryHandler) GetWagerTransactionByExternalID(response http.ResponseWriter, request *http.Request) {
	identity, ok := platformauth.IdentityFromContext(request.Context())
	if !ok || identity.ProviderID == "" {
		writeError(response, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	providerID := request.PathValue("providerId")
	if providerID != identity.ProviderID {
		writeError(response, http.StatusForbidden, "PROVIDER_MISMATCH", "provider identity does not match request")
		return
	}
	transaction, err := handler.executor.GetWagerTransactionByExternalID(
		request.Context(), providerID, request.PathValue("externalTransactionId"),
	)
	if err != nil {
		handler.writeQueryError(response, err)
		return
	}
	handler.writeWagerTransaction(response, transaction)
}

func (handler *WageringQueryHandler) writeWagerTransaction(
	response http.ResponseWriter,
	transaction domain.WagerTransaction,
) {
	body := wagerTransactionDetailsResponse{
		TransactionID: transaction.ID(), ExternalTransactionID: transaction.ExternalTransactionID(),
		ProviderID: transaction.ProviderID(), WalletID: transaction.WalletID(), PlayerID: transaction.PlayerID(),
		RoundID: transaction.RoundID(), GameID: transaction.GameID(), Kind: transaction.Kind(),
		Money: transaction.Money(), Status: transaction.Status(), FailureCode: transaction.FailureCode(),
		ReferenceExternalTransactionID: transaction.ReferenceExternalTransactionID(),
		ReferenceTransactionID:         transaction.ReferenceTransactionID(), CreatedAt: transaction.CreatedAt(),
		UpdatedAt: transaction.UpdatedAt(),
	}
	if balance, ok := transaction.ResultBalance(); ok {
		body.Balance = &balance
	}
	writeJSON(response, http.StatusOK, body)
}

type reconciliationResponse struct {
	WalletID          string       `json:"walletId"`
	StoredBalance     domain.Money `json:"storedBalance"`
	CalculatedBalance domain.Money `json:"calculatedBalance"`
	Difference        domain.Money `json:"difference"`
	Consistent        bool         `json:"consistent"`
	CheckedEntries    int64        `json:"checkedEntries"`
}

func (handler *WageringQueryHandler) ReconcileWallet(response http.ResponseWriter, request *http.Request) {
	result, err := handler.executor.ReconcileWallet(request.Context(), request.PathValue("walletId"))
	if err != nil {
		handler.writeQueryError(response, err)
		return
	}
	handler.metrics.RecordReconciliation(result.Consistent)
	if !result.Consistent {
		slog.Warn("wallet reconciliation divergence",
			"walletId", observability.SafeLogValue(result.WalletID),
			"checkedEntries", result.CheckedEntries)
	}
	writeJSON(response, http.StatusOK, reconciliationResponse(result))
}

func (handler *WageringQueryHandler) writeQueryError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrWalletNotFound):
		writeError(response, http.StatusNotFound, "WALLET_NOT_FOUND", "wallet not found")
	case errors.Is(err, application.ErrWagerTransactionNotFound):
		writeError(response, http.StatusNotFound, "WAGER_TRANSACTION_NOT_FOUND", "wager transaction not found")
	case errors.Is(err, application.ErrInvalidWageringQuery):
		writeError(response, http.StatusBadRequest, "INVALID_QUERY", "invalid query")
	default:
		writeError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

type ledgerCursorPayload struct {
	CreatedAt time.Time `json:"createdAt"`
	EntryID   string    `json:"entryId"`
}

func encodeLedgerCursor(cursor application.LedgerCursor) string {
	payload, _ := json.Marshal(ledgerCursorPayload{CreatedAt: cursor.CreatedAt.UTC(), EntryID: cursor.EntryID})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeLedgerCursor(raw string) (*application.LedgerCursor, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	var decoded ledgerCursorPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	if decoded.CreatedAt.IsZero() || decoded.EntryID == "" {
		return nil, application.ErrInvalidWageringQuery
	}
	return &application.LedgerCursor{CreatedAt: decoded.CreatedAt.UTC(), EntryID: decoded.EntryID}, nil
}
