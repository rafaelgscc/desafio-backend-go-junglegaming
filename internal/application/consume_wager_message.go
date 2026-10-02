package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

const WagerTransactionsConsumerName = "wager-transactions"

var (
	ErrWagerMessageExecutorRequired = errors.New("wager message executor is required")
	ErrInvalidWagerMessage          = errors.New("invalid wager message")
	ErrInboxMessageBusy             = errors.New("inbox message is leased by another worker")
)

type WagerMessageExecutor interface {
	Execute(context.Context, ExecuteWagerTransactionCommand) (ExecuteWagerTransactionResult, error)
}

type WagerTransactionRequestedEnvelope struct {
	MessageID  string                        `json:"messageId"`
	Type       string                        `json:"type"`
	OccurredAt time.Time                     `json:"occurredAt"`
	Data       WagerTransactionRequestedData `json:"data"`
}

type WagerTransactionRequestedData struct {
	ProviderID                     string                      `json:"providerId"`
	ExternalTransactionID          string                      `json:"externalTransactionId"`
	IdempotencyKey                 string                      `json:"idempotencyKey"`
	PlayerID                       string                      `json:"playerId"`
	WalletID                       string                      `json:"walletId"`
	RoundID                        string                      `json:"roundId"`
	GameID                         string                      `json:"gameId"`
	Kind                           domain.WagerTransactionKind `json:"kind"`
	Money                          domain.Money                `json:"money"`
	ReferenceExternalTransactionID string                      `json:"referenceExternalTransactionId,omitempty"`
}

type ConsumeWagerMessageResult struct {
	MessageID        string
	DeleteFromQueue  bool
	AlreadyProcessed bool
}

type ConsumeWagerMessageUseCase struct {
	inbox    InboxRepository
	executor WagerMessageExecutor
}

func NewConsumeWagerMessageUseCase(
	inbox InboxRepository,
	executor WagerMessageExecutor,
) (*ConsumeWagerMessageUseCase, error) {
	if inbox == nil {
		return nil, ErrInboxRepositoryRequired
	}
	if executor == nil {
		return nil, ErrWagerMessageExecutorRequired
	}
	return &ConsumeWagerMessageUseCase{inbox: inbox, executor: executor}, nil
}

func (useCase *ConsumeWagerMessageUseCase) Execute(
	ctx context.Context,
	body []byte,
	workerID string,
	now time.Time,
	leaseExpiresAt time.Time,
	nextAttemptAt time.Time,
) (ConsumeWagerMessageResult, error) {
	envelope, err := decodeWagerMessage(body)
	if err != nil || workerID == "" || now.IsZero() || !leaseExpiresAt.After(now) {
		return ConsumeWagerMessageResult{}, ErrInvalidWagerMessage
	}
	payloadHash := sha256.Sum256(body)
	message := InboxMessage{
		ConsumerName: WagerTransactionsConsumerName,
		MessageID:    envelope.MessageID,
		EventID:      envelope.MessageID,
		EventType:    envelope.Type,
		PayloadHash:  "sha256:" + hex.EncodeToString(payloadHash[:]),
		Payload:      append([]byte(nil), body...),
		ReceivedAt:   now.UTC(),
	}
	claim, err := useCase.inbox.Claim(ctx, message, workerID, leaseExpiresAt.UTC())
	if err != nil {
		return ConsumeWagerMessageResult{MessageID: envelope.MessageID}, err
	}
	switch claim {
	case InboxAlreadyProcessed:
		return ConsumeWagerMessageResult{
			MessageID: envelope.MessageID, DeleteFromQueue: true, AlreadyProcessed: true,
		}, nil
	case InboxBusy:
		return ConsumeWagerMessageResult{MessageID: envelope.MessageID}, ErrInboxMessageBusy
	case InboxClaimed:
	default:
		return ConsumeWagerMessageResult{MessageID: envelope.MessageID}, ErrInvalidWagerMessage
	}

	command := executeCommandFromMessage(envelope, now.UTC())
	command.InboxCompletion = &InboxCompletion{
		ConsumerName: WagerTransactionsConsumerName,
		MessageID:    envelope.MessageID,
		LeaseOwner:   workerID,
		ProcessedAt:  now.UTC(),
	}
	if _, err := useCase.executor.Execute(ctx, command); err != nil {
		failErr := useCase.inbox.Fail(
			ctx, WagerTransactionsConsumerName, envelope.MessageID,
			workerID, nextAttemptAt.UTC(), err.Error(),
		)
		if failErr != nil {
			return ConsumeWagerMessageResult{MessageID: envelope.MessageID},
				fmt.Errorf("process wager message: %w; release inbox: %v", err, failErr)
		}
		return ConsumeWagerMessageResult{MessageID: envelope.MessageID}, err
	}
	if err := useCase.inbox.Complete(
		ctx, WagerTransactionsConsumerName, envelope.MessageID, workerID, now.UTC(),
	); err != nil {
		return ConsumeWagerMessageResult{MessageID: envelope.MessageID}, err
	}
	return ConsumeWagerMessageResult{MessageID: envelope.MessageID, DeleteFromQueue: true}, nil
}

func decodeWagerMessage(body []byte) (WagerTransactionRequestedEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var envelope WagerTransactionRequestedEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return envelope, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return envelope, ErrInvalidWagerMessage
	}
	if envelope.MessageID == "" || envelope.Type != "WagerTransactionRequested" ||
		envelope.OccurredAt.IsZero() || envelope.Data.IdempotencyKey == "" {
		return envelope, ErrInvalidWagerMessage
	}
	return envelope, nil
}

func executeCommandFromMessage(
	envelope WagerTransactionRequestedEnvelope,
	processedAt time.Time,
) ExecuteWagerTransactionCommand {
	stableID := func(suffix string) string {
		digest := sha256.Sum256([]byte(envelope.MessageID + ":" + suffix))
		return hex.EncodeToString(digest[:16])
	}
	return ExecuteWagerTransactionCommand{
		TransactionID: stableID("transaction"), LedgerEntryID: stableID("ledger"),
		OutcomeEventID: stableID("outcome"), BalanceChangedEventID: stableID("balance"),
		IdempotencyKey: envelope.Data.IdempotencyKey,
		Payload: WagerTransactionPayload{
			ProviderID: envelope.Data.ProviderID, ExternalTransactionID: envelope.Data.ExternalTransactionID,
			PlayerID: envelope.Data.PlayerID, WalletID: envelope.Data.WalletID,
			RoundID: envelope.Data.RoundID, GameID: envelope.Data.GameID,
			Kind: envelope.Data.Kind, Money: envelope.Data.Money,
			ReferenceExternalTransactionID: envelope.Data.ReferenceExternalTransactionID,
		},
		CorrelationID: envelope.MessageID, CausationID: envelope.MessageID,
		OccurredAt: processedAt, NextReferenceAttemptAt: processedAt.Add(time.Minute),
	}
}
