package application

import (
	"context"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

// WageringTransaction exposes every persistence operation that must share the
// same database transaction while processing a wager.
type WageringTransaction interface {
	FindWagerTransactionForUpdate(
		ctx context.Context,
		transactionID string,
	) (domain.WagerTransaction, error)
	FindWalletForUpdate(ctx context.Context, walletID string) (domain.Wallet, error)
	SaveWagerTransaction(ctx context.Context, transaction domain.WagerTransaction) error
	SaveWallet(ctx context.Context, wallet domain.Wallet) error
	AppendWalletLedgerEntry(ctx context.Context, entry domain.WalletLedgerEntry) error
}

// WageringUnitOfWork provides the atomic boundary used by wagering use cases.
type WageringUnitOfWork interface {
	WithinTransaction(
		ctx context.Context,
		fn func(WageringTransaction) error,
	) error
}
