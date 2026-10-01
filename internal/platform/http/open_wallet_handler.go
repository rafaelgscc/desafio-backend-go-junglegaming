package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

const maxRequestBodySize = 1 << 20

type OpenWalletExecutor interface {
	Execute(
		ctx context.Context,
		command application.OpenWalletCommand,
	) (application.OpenWalletResult, error)
}

type OpenWalletHandler struct {
	executor    OpenWalletExecutor
	idGenerator IDGenerator
	clock       Clock
}

func NewOpenWalletHandler(
	executor OpenWalletExecutor,
	idGenerator IDGenerator,
	clock Clock,
) (*OpenWalletHandler, error) {
	if executor == nil {
		return nil, ErrOpenWalletExecutorRequired
	}
	if idGenerator == nil {
		return nil, ErrIDGeneratorRequired
	}
	if clock == nil {
		return nil, ErrClockRequired
	}

	return &OpenWalletHandler{
		executor:    executor,
		idGenerator: idGenerator,
		clock:       clock,
	}, nil
}

type openWalletRequest struct {
	PlayerID       string       `json:"playerId"`
	InitialBalance domain.Money `json:"initialBalance"`
}

type openWalletResponse struct {
	ID       string       `json:"id"`
	PlayerID string       `json:"playerId"`
	Balance  domain.Money `json:"balance"`
	Version  int64        `json:"version"`
}

func (handler *OpenWalletHandler) ServeHTTP(
	response http.ResponseWriter,
	request *http.Request,
) {
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
	var payload openWalletRequest
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

	ids := make([]string, 5)
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

	result, err := handler.executor.Execute(request.Context(), application.OpenWalletCommand{
		WalletID:              ids[0],
		PlayerID:              payload.PlayerID,
		InitialBalance:        payload.InitialBalance,
		OpeningTransactionID:  ids[1],
		LedgerEntryID:         ids[2],
		ProcessedEventID:      ids[3],
		BalanceChangedEventID: ids[4],
		CorrelationID:         correlationID,
		CausationID:           request.Header.Get("X-Request-ID"),
		CreatedAt:             handler.clock.Now().UTC(),
	})
	if err != nil {
		handler.writeApplicationError(response, err)
		return
	}

	writeJSON(response, http.StatusCreated, openWalletResponse{
		ID:       result.WalletID,
		PlayerID: result.PlayerID,
		Balance:  result.Balance,
		Version:  result.Version,
	})
}

func (handler *OpenWalletHandler) writeApplicationError(
	response http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(err, application.ErrWalletAlreadyExists):
		writeError(
			response,
			http.StatusConflict,
			"WALLET_ALREADY_EXISTS",
			"wallet already exists for player and currency",
		)
	case isInvalidOpenWalletError(err):
		writeError(response, http.StatusBadRequest, "INVALID_REQUEST", "invalid wallet request")
	default:
		writeError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

func isInvalidOpenWalletError(err error) bool {
	return errors.Is(err, application.ErrInvalidOpenWalletCommand) ||
		errors.Is(err, domain.ErrInvalidWalletID) ||
		errors.Is(err, domain.ErrInvalidPlayerID) ||
		errors.Is(err, domain.ErrInvalidAmount) ||
		errors.Is(err, domain.ErrInvalidCurrency) ||
		errors.Is(err, domain.ErrMoneyOverflow) ||
		errors.Is(err, domain.ErrNegativeBalance) ||
		errors.Is(err, domain.ErrInvalidTimestamp)
}

func hasJSONContentType(request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}
