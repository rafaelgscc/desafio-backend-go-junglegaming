package application

import (
	"context"
	"errors"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

var (
	ErrWalletNotFound         = errors.New("wallet not found")
	ErrConcurrentWalletUpdate = errors.New("wallet was concurrently updated")
)

// WageringTransaction exposes every persistence operation that must share the
// same database transaction while processing a wager.
type WageringTransaction interface {
	FindWagerTransactionForUpdate(
		ctx context.Context,
		transactionID string,
	) (domain.WagerTransaction, error)
	FindWagerTransactionByIdempotencyKeyForUpdate(
		ctx context.Context,
		providerID string,
		idempotencyKey string,
	) (domain.WagerTransaction, error)
	FindWagerTransactionByExternalIDForUpdate(
		ctx context.Context,
		providerID string,
		externalTransactionID string,
	) (domain.WagerTransaction, error)
	FindWalletForUpdate(ctx context.Context, walletID string) (domain.Wallet, error)
	WalletExistsForPlayerAndCurrency(
		ctx context.Context,
		playerID string,
		currency string,
	) (bool, error)
	HasProcessedReversal(
		ctx context.Context,
		referenceTransactionID string,
		kind domain.WagerTransactionKind,
	) (bool, error)
	InsertWagerTransaction(ctx context.Context, transaction domain.WagerTransaction) error
	InsertWallet(ctx context.Context, wallet domain.Wallet) error
	SaveWagerTransaction(ctx context.Context, transaction domain.WagerTransaction) error
	SaveWallet(ctx context.Context, wallet domain.Wallet) error
	AppendWalletLedgerEntry(ctx context.Context, entry domain.WalletLedgerEntry) error
	AppendOutboxEvent(ctx context.Context, event IntegrationEvent) error
}

// WageringUnitOfWork provides the atomic boundary used by wagering use cases.
type WageringUnitOfWork interface {
	WithinTransaction(
		ctx context.Context,
		fn func(WageringTransaction) error,
	) error
}
