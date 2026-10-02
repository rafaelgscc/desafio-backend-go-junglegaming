package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
	platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/observability"
)

const referenceRetryDelay = time.Minute

type ExecuteWagerTransactionExecutor interface {
	Execute(
		context.Context,
		application.ExecuteWagerTransactionCommand,
	) (application.ExecuteWagerTransactionResult, error)
}

type WagerTransactionHandler struct {
	executor    ExecuteWagerTransactionExecutor
	idGenerator IDGenerator
	clock       Clock
	metrics     *observability.Metrics
}

func NewWagerTransactionHandler(
	executor ExecuteWagerTransactionExecutor,
	idGenerator IDGenerator,
	clock Clock,
	metrics *observability.Metrics,
) (*WagerTransactionHandler, error) {
	if executor == nil {
		return nil, ErrExecuteWagerTransactionExecutorRequired
	}
	if idGenerator == nil {
		return nil, ErrIDGeneratorRequired
	}
	if clock == nil {
		return nil, ErrClockRequired
	}
	if metrics == nil {
		return nil, observability.ErrMetricsRequired
	}
	return &WagerTransactionHandler{
		executor: executor, idGenerator: idGenerator, clock: clock, metrics: metrics,
	}, nil
}

type wagerTransactionRequest struct {
	ProviderID                     string                      `json:"providerId"`
	ExternalTransactionID          string                      `json:"externalTransactionId"`
	PlayerID                       string                      `json:"playerId"`
	WalletID                       string                      `json:"walletId"`
	RoundID                        string                      `json:"roundId"`
	GameID                         string                      `json:"gameId"`
	Kind                           domain.WagerTransactionKind `json:"kind"`
	Money                          domain.Money                `json:"money"`
	ReferenceExternalTransactionID string                      `json:"referenceExternalTransactionId,omitempty"`
}

type wagerTransactionResponse struct {
	TransactionID    string                             `json:"transactionId"`
	Status           domain.WagerTransactionStatus      `json:"status"`
	Balance          *domain.Money                      `json:"balance,omitempty"`
	FailureCode      domain.WagerTransactionFailureCode `json:"failureCode,omitempty"`
	IdempotentReplay bool                               `json:"idempotentReplay"`
}

func (handler *WagerTransactionHandler) ServeHTTP(
	response http.ResponseWriter,
	request *http.Request,
) {
	identity, ok := platformauth.IdentityFromContext(request.Context())
	if !ok || identity.ProviderID == "" {
		writeError(response, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	idempotencyKey := request.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		writeError(
			response,
			http.StatusBadRequest,
			"IDEMPOTENCY_KEY_REQUIRED",
			"Idempotency-Key header is required",
		)
		return
	}
	if !hasJSONContentType(request) {
		writeError(
			response,
			http.StatusUnsupportedMediaType,
			"UNSUPPORTED_MEDIA_TYPE",
			"Content-Type must be application/json",
		)
		return
	}

	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBodySize)
	var payload wagerTransactionRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if payload.ProviderID != identity.ProviderID {
		writeError(response, http.StatusForbidden, "PROVIDER_MISMATCH", "provider identity does not match request")
		return
	}

	ids := make([]string, 4)
	for index := range ids {
		id, err := handler.idGenerator.NewID()
		if err != nil {
			writeError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			return
		}
		ids[index] = id
	}
	correlationID := request.Header.Get("X-Correlation-ID")
	if correlationID == "" {
		var err error
		correlationID, err = handler.idGenerator.NewID()
		if err != nil {
			writeError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			return
		}
	}
	now := handler.clock.Now().UTC()
	startedAt := time.Now()
	result, err := handler.executor.Execute(
		request.Context(),
		application.ExecuteWagerTransactionCommand{
			TransactionID: ids[0], LedgerEntryID: ids[1],
			OutcomeEventID: ids[2], BalanceChangedEventID: ids[3],
			IdempotencyKey: idempotencyKey,
			Payload: application.WagerTransactionPayload{
				ProviderID: identity.ProviderID, ExternalTransactionID: payload.ExternalTransactionID,
				PlayerID: payload.PlayerID, WalletID: payload.WalletID,
				RoundID: payload.RoundID, GameID: payload.GameID, Kind: payload.Kind,
				Money:                          payload.Money,
				ReferenceExternalTransactionID: payload.ReferenceExternalTransactionID,
			},
			CorrelationID: correlationID, CausationID: request.Header.Get("X-Request-ID"),
			OccurredAt: now, NextReferenceAttemptAt: now.Add(referenceRetryDelay),
		},
	)
	handler.metrics.ObserveProcessing("http", time.Since(startedAt))
	if err != nil {
		handler.metrics.RecordWagerResult("ERROR")
		if errors.Is(err, application.ErrConcurrentWalletUpdate) {
			handler.metrics.RecordConcurrencyConflict()
		}
		slog.Error("wager transaction failed",
			"correlationId", observability.SafeLogValue(correlationID),
			"transactionId", ids[0], "walletId", observability.SafeLogValue(payload.WalletID),
			"providerId", observability.SafeLogValue(identity.ProviderID), "error", err)
		handler.writeApplicationError(response, err)
		return
	}
	handler.metrics.RecordWagerResult(string(result.Status))
	if result.IdempotentReplay {
		handler.metrics.RecordDuplicate("http")
	}
	slog.Info("wager transaction processed",
		"correlationId", observability.SafeLogValue(correlationID),
		"transactionId", result.TransactionID, "walletId", observability.SafeLogValue(payload.WalletID),
		"providerId", observability.SafeLogValue(identity.ProviderID), "status", result.Status,
		"idempotentReplay", result.IdempotentReplay)

	httpStatus := statusForWagerTransactionResult(result.Status)
	body := wagerTransactionResponse{
		TransactionID:    result.TransactionID,
		Status:           result.Status,
		FailureCode:      result.FailureCode,
		IdempotentReplay: result.IdempotentReplay,
	}
	if result.HasBalance {
		balance := result.Balance
		body.Balance = &balance
	}
	writeJSON(response, httpStatus, body)
}

func statusForWagerTransactionResult(status domain.WagerTransactionStatus) int {
	switch status {
	case domain.WagerTransactionStatusPending,
		domain.WagerTransactionStatusPendingReference:
		return http.StatusAccepted
	case domain.WagerTransactionStatusRejected:
		return http.StatusUnprocessableEntity
	case domain.WagerTransactionStatusFailed:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}

func (handler *WagerTransactionHandler) writeApplicationError(
	response http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(err, application.ErrIdempotencyKeyConflict):
		writeError(response, http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "idempotency key was used with different content")
	case errors.Is(err, application.ErrExternalTransactionConflict):
		writeError(response, http.StatusConflict, "EXTERNAL_TRANSACTION_CONFLICT", "external transaction already exists")
	case errors.Is(err, application.ErrWalletNotFound):
		writeError(response, http.StatusNotFound, "WALLET_NOT_FOUND", "wallet not found")
	case errors.Is(err, application.ErrConcurrentWalletUpdate):
		writeError(response, http.StatusServiceUnavailable, "TEMPORARILY_UNAVAILABLE", "temporarily unavailable")
	case isInvalidWagerTransactionError(err):
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "invalid wager transaction request")
	default:
		writeError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

func isInvalidWagerTransactionError(err error) bool {
	return errors.Is(err, application.ErrInvalidExecuteWagerTransactionCommand) ||
		errors.Is(err, application.ErrInvalidProcessWagerCommand) ||
		errors.Is(err, application.ErrInvalidProcessReversalCommand) ||
		errors.Is(err, application.ErrWagerWalletMismatch) ||
		errors.Is(err, application.ErrWagerPlayerMismatch) ||
		errors.Is(err, domain.ErrInvalidWagerTransactionID) ||
		errors.Is(err, domain.ErrInvalidExternalTransactionID) ||
		errors.Is(err, domain.ErrInvalidProviderID) ||
		errors.Is(err, domain.ErrInvalidIdempotencyKey) ||
		errors.Is(err, domain.ErrInvalidPayloadHash) ||
		errors.Is(err, domain.ErrInvalidWalletID) ||
		errors.Is(err, domain.ErrInvalidPlayerID) ||
		errors.Is(err, domain.ErrInvalidRoundID) ||
		errors.Is(err, domain.ErrInvalidGameID) ||
		errors.Is(err, domain.ErrInvalidWagerTransactionKind) ||
		errors.Is(err, domain.ErrInvalidAmount) ||
		errors.Is(err, domain.ErrInvalidCurrency) ||
		errors.Is(err, domain.ErrCurrencyMismatch) ||
		errors.Is(err, domain.ErrMoneyOverflow) ||
		errors.Is(err, domain.ErrInvalidTimestamp) ||
		errors.Is(err, domain.ErrNonPositiveAmount) ||
		errors.Is(err, domain.ErrLossAmountMustBeZero) ||
		errors.Is(err, domain.ErrReferenceExternalTransactionIDRequired) ||
		errors.Is(err, domain.ErrUnexpectedReferenceExternalTransactionID)
}
