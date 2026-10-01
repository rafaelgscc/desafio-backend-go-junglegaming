package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

func TestHashWagerTransactionPayloadIsCanonical(t *testing.T) {
	t.Parallel()

	money, err := domain.NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() setup error: %v", err)
	}
	payload := WagerTransactionPayload{
		ProviderID: "provider-a", ExternalTransactionID: "transaction-1",
		PlayerID: "player-1", WalletID: "wallet-1", RoundID: "round-1",
		GameID: "game-1", Kind: domain.WagerTransactionKindBet, Money: money,
	}

	hashA, err := HashWagerTransactionPayload(payload)
	if err != nil {
		t.Fatalf("HashWagerTransactionPayload() unexpected error: %v", err)
	}
	hashB, err := HashWagerTransactionPayload(payload)
	if err != nil {
		t.Fatalf("HashWagerTransactionPayload() unexpected error: %v", err)
	}
	if hashA != hashB {
		t.Fatalf("equal payload hashes differ: %q != %q", hashA, hashB)
	}
	if !strings.HasPrefix(hashA, "sha256:") || len(hashA) != len("sha256:")+64 {
		t.Fatalf("hash = %q, want sha256 followed by 64 hexadecimal characters", hashA)
	}

	payload.RoundID = "another-round"
	hashDifferent, err := HashWagerTransactionPayload(payload)
	if err != nil {
		t.Fatalf("HashWagerTransactionPayload() unexpected error: %v", err)
	}
	if hashDifferent == hashA {
		t.Fatal("different business payload produced the same hash")
	}
}

func TestExecuteWagerTransactionDispatchesAllExternalKinds(t *testing.T) {
	t.Parallel()

	for _, kind := range []domain.WagerTransactionKind{
		domain.WagerTransactionKindBet,
		domain.WagerTransactionKindWin,
		domain.WagerTransactionKindLoss,
		domain.WagerTransactionKindRefund,
		domain.WagerTransactionKindRollback,
	} {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			calls := map[domain.WagerTransactionKind]int{}
			balance, err := domain.NewMoney("75.00", "BRL")
			if err != nil {
				t.Fatalf("NewMoney() setup error: %v", err)
			}
			submit := submitWagerExecutorFunc(func(
				_ context.Context,
				command SubmitWagerTransactionCommand,
			) (SubmitWagerTransactionResult, error) {
				if command.PayloadHash == "" {
					t.Fatal("submit command did not receive payload hash")
				}
				return SubmitWagerTransactionResult{
					TransactionID: command.TransactionID,
					Status:        domain.WagerTransactionStatusPending,
				}, nil
			})
			process := func(processedKind domain.WagerTransactionKind) ProcessWagerResult {
				calls[processedKind]++
				return ProcessWagerResult{
					Status:  domain.WagerTransactionStatusProcessed,
					Balance: balance,
				}
			}
			useCase := newExecuteWagerUseCaseForTest(t, submit, calls, process)

			command := executeWagerTransactionCommandFixture(t, kind)
			result, err := useCase.Execute(context.Background(), command)
			if err != nil {
				t.Fatalf("Execute() unexpected error: %v", err)
			}
			if calls[kind] != 1 || totalKindCalls(calls) != 1 {
				t.Fatalf("processor calls = %#v, want one call for %s", calls, kind)
			}
			if result.Status != domain.WagerTransactionStatusProcessed ||
				!result.HasBalance || result.Balance.Amount() != "75.00" ||
				result.IdempotentReplay {
				t.Fatalf("result = %#v, want newly processed transaction", result)
			}
		})
	}
}

func TestExecuteWagerTransactionReturnsTerminalReplayWithoutProcessing(t *testing.T) {
	t.Parallel()

	balance, err := domain.NewMoney("80.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() setup error: %v", err)
	}
	calls := map[domain.WagerTransactionKind]int{}
	submit := submitWagerExecutorFunc(func(
		context.Context,
		SubmitWagerTransactionCommand,
	) (SubmitWagerTransactionResult, error) {
		return SubmitWagerTransactionResult{
			TransactionID:    "persisted-transaction",
			Status:           domain.WagerTransactionStatusProcessed,
			Balance:          balance,
			HasBalance:       true,
			IdempotentReplay: true,
		}, nil
	})
	useCase := newExecuteWagerUseCaseForTest(t, submit, calls, func(
		domain.WagerTransactionKind,
	) ProcessWagerResult {
		t.Fatal("processor called for terminal replay")
		return ProcessWagerResult{}
	})

	result, err := useCase.Execute(
		context.Background(),
		executeWagerTransactionCommandFixture(t, domain.WagerTransactionKindBet),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.TransactionID != "persisted-transaction" || !result.IdempotentReplay ||
		!result.HasBalance || result.Balance.Amount() != "80.00" {
		t.Fatalf("result = %#v, want original persisted result", result)
	}
}

func TestExecuteWagerTransactionRejectsOpening(t *testing.T) {
	t.Parallel()

	useCase := newExecuteWagerUseCaseForTest(
		t,
		submitWagerExecutorFunc(func(context.Context, SubmitWagerTransactionCommand) (SubmitWagerTransactionResult, error) {
			t.Fatal("submit called for OPENING")
			return SubmitWagerTransactionResult{}, nil
		}),
		map[domain.WagerTransactionKind]int{},
		func(domain.WagerTransactionKind) ProcessWagerResult { return ProcessWagerResult{} },
	)
	command := executeWagerTransactionCommandFixture(t, domain.WagerTransactionKindBet)
	command.Payload.Kind = domain.WagerTransactionKindOpening

	_, err := useCase.Execute(context.Background(), command)
	if !errors.Is(err, ErrInvalidExecuteWagerTransactionCommand) {
		t.Fatalf("Execute() error = %v, want ErrInvalidExecuteWagerTransactionCommand", err)
	}
}

type submitWagerExecutorFunc func(
	context.Context,
	SubmitWagerTransactionCommand,
) (SubmitWagerTransactionResult, error)

func (function submitWagerExecutorFunc) Execute(
	ctx context.Context,
	command SubmitWagerTransactionCommand,
) (SubmitWagerTransactionResult, error) {
	return function(ctx, command)
}

type betProcessorFunc func(context.Context, ProcessBetCommand) (ProcessBetResult, error)

func (function betProcessorFunc) Execute(ctx context.Context, command ProcessBetCommand) (ProcessBetResult, error) {
	return function(ctx, command)
}

type winProcessorFunc func(context.Context, ProcessWinCommand) (ProcessWinResult, error)

func (function winProcessorFunc) Execute(ctx context.Context, command ProcessWinCommand) (ProcessWinResult, error) {
	return function(ctx, command)
}

type lossProcessorFunc func(context.Context, ProcessLossCommand) (ProcessLossResult, error)

func (function lossProcessorFunc) Execute(ctx context.Context, command ProcessLossCommand) (ProcessLossResult, error) {
	return function(ctx, command)
}

type reversalProcessorFunc func(context.Context, ProcessReversalCommand) (ProcessReversalResult, error)

func (function reversalProcessorFunc) Execute(ctx context.Context, command ProcessReversalCommand) (ProcessReversalResult, error) {
	return function(ctx, command)
}

func newExecuteWagerUseCaseForTest(
	t *testing.T,
	submit WagerTransactionSubmitter,
	calls map[domain.WagerTransactionKind]int,
	process func(domain.WagerTransactionKind) ProcessWagerResult,
) *ExecuteWagerTransactionUseCase {
	t.Helper()
	useCase, err := NewExecuteWagerTransactionUseCase(
		submit,
		betProcessorFunc(func(context.Context, ProcessBetCommand) (ProcessBetResult, error) {
			return process(domain.WagerTransactionKindBet), nil
		}),
		winProcessorFunc(func(context.Context, ProcessWinCommand) (ProcessWinResult, error) {
			return process(domain.WagerTransactionKindWin), nil
		}),
		lossProcessorFunc(func(context.Context, ProcessLossCommand) (ProcessLossResult, error) {
			return process(domain.WagerTransactionKindLoss), nil
		}),
		reversalProcessorFunc(func(context.Context, ProcessReversalCommand) (ProcessReversalResult, error) {
			return process(domain.WagerTransactionKindRefund), nil
		}),
		reversalProcessorFunc(func(context.Context, ProcessReversalCommand) (ProcessReversalResult, error) {
			return process(domain.WagerTransactionKindRollback), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewExecuteWagerTransactionUseCase() unexpected error: %v", err)
	}
	return useCase
}

func executeWagerTransactionCommandFixture(
	t *testing.T,
	kind domain.WagerTransactionKind,
) ExecuteWagerTransactionCommand {
	t.Helper()
	amount := "25.00"
	if kind == domain.WagerTransactionKindLoss {
		amount = "0.00"
	}
	money, err := domain.NewMoney(amount, "BRL")
	if err != nil {
		t.Fatalf("NewMoney() setup error: %v", err)
	}
	reference := ""
	if kind == domain.WagerTransactionKindRefund || kind == domain.WagerTransactionKindRollback {
		reference = "external-bet-1"
	}
	now := time.Date(2026, time.October, 1, 18, 0, 0, 0, time.UTC)
	return ExecuteWagerTransactionCommand{
		TransactionID: "transaction-1", LedgerEntryID: "ledger-1",
		OutcomeEventID: "outcome-1", BalanceChangedEventID: "balance-event-1",
		IdempotencyKey: "provider-a:external-1",
		Payload: WagerTransactionPayload{
			ProviderID: "provider-a", ExternalTransactionID: "external-1",
			PlayerID: "player-1", WalletID: "wallet-1", RoundID: "round-1",
			GameID: "game-1", Kind: kind, Money: money,
			ReferenceExternalTransactionID: reference,
		},
		CorrelationID: "correlation-1", CausationID: "request-1",
		OccurredAt: now, NextReferenceAttemptAt: now.Add(time.Minute),
	}
}

func totalKindCalls(calls map[domain.WagerTransactionKind]int) int {
	total := 0
	for _, count := range calls {
		total += count
	}
	return total
}
