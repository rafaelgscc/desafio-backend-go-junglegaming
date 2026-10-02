package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

var (
	ErrInvalidExecuteWagerTransactionCommand = errors.New("invalid execute wager transaction command")
	ErrWagerTransactionExecutorRequired      = errors.New("wager transaction executor is required")
)

type WagerTransactionPayload struct {
	ProviderID                     string
	ExternalTransactionID          string
	PlayerID                       string
	WalletID                       string
	RoundID                        string
	GameID                         string
	Kind                           domain.WagerTransactionKind
	Money                          domain.Money
	ReferenceExternalTransactionID string
}

type canonicalWagerTransactionPayload struct {
	ExternalTransactionID          string                      `json:"externalTransactionId"`
	GameID                         string                      `json:"gameId"`
	Kind                           domain.WagerTransactionKind `json:"kind"`
	Money                          domain.Money                `json:"money"`
	PlayerID                       string                      `json:"playerId"`
	ProviderID                     string                      `json:"providerId"`
	ReferenceExternalTransactionID string                      `json:"referenceExternalTransactionId,omitempty"`
	RoundID                        string                      `json:"roundId"`
	WalletID                       string                      `json:"walletId"`
}

func HashWagerTransactionPayload(payload WagerTransactionPayload) (string, error) {
	canonical, err := json.Marshal(canonicalWagerTransactionPayload{
		ExternalTransactionID:          payload.ExternalTransactionID,
		GameID:                         payload.GameID,
		Kind:                           payload.Kind,
		Money:                          payload.Money,
		PlayerID:                       payload.PlayerID,
		ProviderID:                     payload.ProviderID,
		ReferenceExternalTransactionID: payload.ReferenceExternalTransactionID,
		RoundID:                        payload.RoundID,
		WalletID:                       payload.WalletID,
	})
	if err != nil {
		return "", fmt.Errorf("encode canonical wager transaction payload: %w", err)
	}

	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

type ExecuteWagerTransactionCommand struct {
	TransactionID          string
	LedgerEntryID          string
	OutcomeEventID         string
	BalanceChangedEventID  string
	IdempotencyKey         string
	Payload                WagerTransactionPayload
	CorrelationID          string
	CausationID            string
	OccurredAt             time.Time
	NextReferenceAttemptAt time.Time
	InboxCompletion        *InboxCompletion
}

type ExecuteWagerTransactionResult struct {
	TransactionID    string
	Status           domain.WagerTransactionStatus
	Balance          domain.Money
	HasBalance       bool
	FailureCode      domain.WagerTransactionFailureCode
	IdempotentReplay bool
}

type WagerTransactionSubmitter interface {
	Execute(context.Context, SubmitWagerTransactionCommand) (SubmitWagerTransactionResult, error)
}

type BetProcessor interface {
	Execute(context.Context, ProcessBetCommand) (ProcessBetResult, error)
}

type WinProcessor interface {
	Execute(context.Context, ProcessWinCommand) (ProcessWinResult, error)
}

type LossProcessor interface {
	Execute(context.Context, ProcessLossCommand) (ProcessLossResult, error)
}

type ReversalProcessor interface {
	Execute(context.Context, ProcessReversalCommand) (ProcessReversalResult, error)
}

type ExecuteWagerTransactionUseCase struct {
	submitter WagerTransactionSubmitter
	bet       BetProcessor
	win       WinProcessor
	loss      LossProcessor
	refund    ReversalProcessor
	rollback  ReversalProcessor
}

func NewExecuteWagerTransactionUseCase(
	submitter WagerTransactionSubmitter,
	bet BetProcessor,
	win WinProcessor,
	loss LossProcessor,
	refund ReversalProcessor,
	rollback ReversalProcessor,
) (*ExecuteWagerTransactionUseCase, error) {
	if submitter == nil || bet == nil || win == nil || loss == nil ||
		refund == nil || rollback == nil {
		return nil, ErrWagerTransactionExecutorRequired
	}

	return &ExecuteWagerTransactionUseCase{
		submitter: submitter,
		bet:       bet,
		win:       win,
		loss:      loss,
		refund:    refund,
		rollback:  rollback,
	}, nil
}

func (useCase *ExecuteWagerTransactionUseCase) Execute(
	ctx context.Context,
	command ExecuteWagerTransactionCommand,
) (ExecuteWagerTransactionResult, error) {
	if !validExecuteWagerTransactionCommand(command) {
		return ExecuteWagerTransactionResult{}, ErrInvalidExecuteWagerTransactionCommand
	}

	payloadHash, err := HashWagerTransactionPayload(command.Payload)
	if err != nil {
		return ExecuteWagerTransactionResult{}, err
	}
	submitCommand := SubmitWagerTransactionCommand{
		TransactionID:                  command.TransactionID,
		ExternalTransactionID:          command.Payload.ExternalTransactionID,
		ProviderID:                     command.Payload.ProviderID,
		IdempotencyKey:                 command.IdempotencyKey,
		PayloadHash:                    payloadHash,
		WalletID:                       command.Payload.WalletID,
		PlayerID:                       command.Payload.PlayerID,
		RoundID:                        command.Payload.RoundID,
		GameID:                         command.Payload.GameID,
		Kind:                           command.Payload.Kind,
		Money:                          command.Payload.Money,
		ReferenceExternalTransactionID: command.Payload.ReferenceExternalTransactionID,
		OccurredAt:                     command.OccurredAt,
	}

	submitted, err := useCase.submitter.Execute(ctx, submitCommand)
	if err != nil {
		return ExecuteWagerTransactionResult{}, err
	}
	if submitted.Status != domain.WagerTransactionStatusPending {
		return executeResultFromSubmission(submitted), nil
	}

	processed, err := useCase.process(ctx, command, submitted.TransactionID)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidWagerTransactionTransition) {
			reloaded, reloadErr := useCase.submitter.Execute(ctx, submitCommand)
			if reloadErr == nil && reloaded.Status != domain.WagerTransactionStatusPending {
				result := executeResultFromSubmission(reloaded)
				result.IdempotentReplay = submitted.IdempotentReplay
				return result, nil
			}
		}
		return ExecuteWagerTransactionResult{}, err
	}

	return ExecuteWagerTransactionResult{
		TransactionID:    submitted.TransactionID,
		Status:           processed.Status,
		Balance:          processed.Balance,
		HasBalance:       true,
		FailureCode:      processed.FailureCode,
		IdempotentReplay: submitted.IdempotentReplay,
	}, nil
}

func (useCase *ExecuteWagerTransactionUseCase) process(
	ctx context.Context,
	command ExecuteWagerTransactionCommand,
	transactionID string,
) (ProcessWagerResult, error) {
	switch command.Payload.Kind {
	case domain.WagerTransactionKindBet:
		return useCase.bet.Execute(ctx, ProcessBetCommand{
			TransactionID: transactionID, LedgerEntryID: command.LedgerEntryID,
			OutcomeEventID:        command.OutcomeEventID,
			BalanceChangedEventID: command.BalanceChangedEventID,
			CorrelationID:         command.CorrelationID, CausationID: command.CausationID,
			ProcessedAt:     command.OccurredAt,
			InboxCompletion: command.InboxCompletion,
		})
	case domain.WagerTransactionKindWin:
		return useCase.win.Execute(ctx, ProcessWinCommand{
			TransactionID: transactionID, LedgerEntryID: command.LedgerEntryID,
			OutcomeEventID:        command.OutcomeEventID,
			BalanceChangedEventID: command.BalanceChangedEventID,
			CorrelationID:         command.CorrelationID, CausationID: command.CausationID,
			ProcessedAt:     command.OccurredAt,
			InboxCompletion: command.InboxCompletion,
		})
	case domain.WagerTransactionKindLoss:
		return useCase.loss.Execute(ctx, ProcessLossCommand{
			TransactionID: transactionID, OutcomeEventID: command.OutcomeEventID,
			CorrelationID: command.CorrelationID, CausationID: command.CausationID,
			ProcessedAt:     command.OccurredAt,
			InboxCompletion: command.InboxCompletion,
		})
	case domain.WagerTransactionKindRefund, domain.WagerTransactionKindRollback:
		processor := useCase.refund
		if command.Payload.Kind == domain.WagerTransactionKindRollback {
			processor = useCase.rollback
		}
		return processor.Execute(ctx, ProcessReversalCommand{
			TransactionID: transactionID, LedgerEntryID: command.LedgerEntryID,
			OutcomeEventID:        command.OutcomeEventID,
			BalanceChangedEventID: command.BalanceChangedEventID,
			CorrelationID:         command.CorrelationID, CausationID: command.CausationID,
			ProcessedAt:            command.OccurredAt,
			NextReferenceAttemptAt: command.NextReferenceAttemptAt,
			InboxCompletion:        command.InboxCompletion,
		})
	default:
		return ProcessWagerResult{}, ErrInvalidExecuteWagerTransactionCommand
	}
}

func validExecuteWagerTransactionCommand(command ExecuteWagerTransactionCommand) bool {
	if command.TransactionID == "" || command.LedgerEntryID == "" ||
		command.OutcomeEventID == "" || command.BalanceChangedEventID == "" ||
		command.IdempotencyKey == "" || command.CorrelationID == "" ||
		command.OccurredAt.IsZero() {
		return false
	}
	switch command.Payload.Kind {
	case domain.WagerTransactionKindBet,
		domain.WagerTransactionKindWin,
		domain.WagerTransactionKindLoss:
		return true
	case domain.WagerTransactionKindRefund,
		domain.WagerTransactionKindRollback:
		return !command.NextReferenceAttemptAt.IsZero() &&
			command.NextReferenceAttemptAt.After(command.OccurredAt)
	default:
		return false
	}
}

func executeResultFromSubmission(
	submitted SubmitWagerTransactionResult,
) ExecuteWagerTransactionResult {
	return ExecuteWagerTransactionResult{
		TransactionID:    submitted.TransactionID,
		Status:           submitted.Status,
		Balance:          submitted.Balance,
		HasBalance:       submitted.HasBalance,
		FailureCode:      submitted.FailureCode,
		IdempotentReplay: submitted.IdempotentReplay,
	}
}
