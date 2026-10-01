package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

const (
	DefaultLedgerPageSize = 50
	MaxLedgerPageSize     = 100
)

var (
	ErrWageringQueryRepositoryRequired = errors.New("wagering query repository is required")
	ErrInvalidWageringQuery            = errors.New("invalid wagering query")
)

type LedgerCursor struct {
	CreatedAt time.Time
	EntryID   string
}

type WalletLedgerPage struct {
	Entries    []domain.WalletLedgerEntry
	NextCursor *LedgerCursor
}

type ReconciliationResult struct {
	WalletID          string
	StoredBalance     domain.Money
	CalculatedBalance domain.Money
	Difference        domain.Money
	Consistent        bool
	CheckedEntries    int64
}

type WageringQueryRepository interface {
	FindWallet(context.Context, string) (domain.Wallet, error)
	ListWalletLedger(
		context.Context,
		string,
		*LedgerCursor,
		int,
	) ([]domain.WalletLedgerEntry, error)
	FindWagerTransactionForProvider(
		context.Context,
		string,
		string,
	) (domain.WagerTransaction, error)
	FindWagerTransactionByExternalID(
		context.Context,
		string,
		string,
	) (domain.WagerTransaction, error)
	ReconcileWallet(context.Context, string) (ReconciliationResult, error)
}

type WageringQueryService struct {
	repository WageringQueryRepository
}

func NewWageringQueryService(
	repository WageringQueryRepository,
) (*WageringQueryService, error) {
	if repository == nil {
		return nil, ErrWageringQueryRepositoryRequired
	}
	return &WageringQueryService{repository: repository}, nil
}

func (service *WageringQueryService) GetWallet(
	ctx context.Context,
	walletID string,
) (domain.Wallet, error) {
	if strings.TrimSpace(walletID) == "" {
		return domain.Wallet{}, ErrInvalidWageringQuery
	}
	return service.repository.FindWallet(ctx, walletID)
}

func (service *WageringQueryService) ListWalletLedger(
	ctx context.Context,
	walletID string,
	cursor *LedgerCursor,
	limit int,
) (WalletLedgerPage, error) {
	if strings.TrimSpace(walletID) == "" || limit < 1 || limit > MaxLedgerPageSize {
		return WalletLedgerPage{}, ErrInvalidWageringQuery
	}
	if cursor != nil && (cursor.CreatedAt.IsZero() || cursor.EntryID == "") {
		return WalletLedgerPage{}, ErrInvalidWageringQuery
	}
	if _, err := service.repository.FindWallet(ctx, walletID); err != nil {
		return WalletLedgerPage{}, err
	}

	entries, err := service.repository.ListWalletLedger(ctx, walletID, cursor, limit+1)
	if err != nil {
		return WalletLedgerPage{}, err
	}

	page := WalletLedgerPage{Entries: entries}
	if len(entries) > limit {
		page.Entries = entries[:limit]
		last := page.Entries[len(page.Entries)-1]
		page.NextCursor = &LedgerCursor{CreatedAt: last.CreatedAt(), EntryID: last.ID()}
	}
	return page, nil
}

func (service *WageringQueryService) GetWagerTransaction(
	ctx context.Context,
	providerID string,
	transactionID string,
) (domain.WagerTransaction, error) {
	if strings.TrimSpace(providerID) == "" || strings.TrimSpace(transactionID) == "" {
		return domain.WagerTransaction{}, ErrInvalidWageringQuery
	}
	return service.repository.FindWagerTransactionForProvider(ctx, providerID, transactionID)
}

func (service *WageringQueryService) GetWagerTransactionByExternalID(
	ctx context.Context,
	providerID string,
	externalTransactionID string,
) (domain.WagerTransaction, error) {
	if strings.TrimSpace(providerID) == "" || strings.TrimSpace(externalTransactionID) == "" {
		return domain.WagerTransaction{}, ErrInvalidWageringQuery
	}
	return service.repository.FindWagerTransactionByExternalID(
		ctx,
		providerID,
		externalTransactionID,
	)
}

func (service *WageringQueryService) ReconcileWallet(
	ctx context.Context,
	walletID string,
) (ReconciliationResult, error) {
	if strings.TrimSpace(walletID) == "" {
		return ReconciliationResult{}, ErrInvalidWageringQuery
	}
	return service.repository.ReconcileWallet(ctx, walletID)
}
