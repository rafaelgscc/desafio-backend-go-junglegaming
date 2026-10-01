package application

import (
	"context"
	"errors"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

var (
	ErrInvalidOpenWalletCommand = errors.New("invalid open wallet command")
	ErrWalletAlreadyExists      = errors.New("wallet already exists for player and currency")
)

type OpenWalletCommand struct {
	WalletID              string
	PlayerID              string
	InitialBalance        domain.Money
	OpeningTransactionID  string
	LedgerEntryID         string
	ProcessedEventID      string
	BalanceChangedEventID string
	CorrelationID         string
	CausationID           string
	CreatedAt             time.Time
}

type OpenWalletResult struct {
	WalletID string
	PlayerID string
	Balance  domain.Money
	Version  int64
}

type OpenWalletUseCase struct {
	unitOfWork WageringUnitOfWork
}

func NewOpenWalletUseCase(unitOfWork WageringUnitOfWork) (*OpenWalletUseCase, error) {
	if unitOfWork == nil {
		return nil, ErrNilWageringUnitOfWork
	}
	return &OpenWalletUseCase{unitOfWork: unitOfWork}, nil
}

func (useCase *OpenWalletUseCase) Execute(
	ctx context.Context,
	command OpenWalletCommand,
) (OpenWalletResult, error) {
	wallet, err := domain.NewWallet(
		command.WalletID,
		command.PlayerID,
		command.InitialBalance,
		command.CreatedAt,
	)
	if err != nil {
		return OpenWalletResult{}, err
	}

	zero, err := domain.ZeroMoney(wallet.Currency())
	if err != nil {
		return OpenWalletResult{}, err
	}
	comparison, err := wallet.Balance().Compare(zero)
	if err != nil {
		return OpenWalletResult{}, err
	}
	withOpening := comparison > 0
	if withOpening && (command.OpeningTransactionID == "" ||
		command.LedgerEntryID == "" || command.ProcessedEventID == "" ||
		command.BalanceChangedEventID == "" || command.CorrelationID == "") {
		return OpenWalletResult{}, ErrInvalidOpenWalletCommand
	}

	err = useCase.unitOfWork.WithinTransaction(ctx, func(tx WageringTransaction) error {
		exists, err := tx.WalletExistsForPlayerAndCurrency(
			ctx,
			wallet.PlayerID(),
			wallet.Currency(),
		)
		if err != nil {
			return err
		}
		if exists {
			return ErrWalletAlreadyExists
		}
		if err := tx.InsertWallet(ctx, wallet); err != nil {
			return err
		}
		if !withOpening {
			return nil
		}

		opening, err := domain.NewOpeningWagerTransaction(
			domain.NewOpeningWagerTransactionParams{
				ID:         command.OpeningTransactionID,
				WalletID:   wallet.ID(),
				PlayerID:   wallet.PlayerID(),
				Money:      wallet.Balance(),
				OccurredAt: command.CreatedAt,
			},
		)
		if err != nil {
			return err
		}
		entry, err := domain.NewWalletLedgerEntry(
			command.LedgerEntryID,
			wallet.ID(),
			opening.ID(),
			domain.LedgerDirectionCredit,
			wallet.Balance(),
			zero,
			wallet.Balance(),
			command.CreatedAt,
		)
		if err != nil {
			return err
		}

		if err := tx.InsertWagerTransaction(ctx, opening); err != nil {
			return err
		}
		if err := tx.AppendWalletLedgerEntry(ctx, entry); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, newWagerTransactionProcessedEvent(
			command.ProcessedEventID,
			command.CorrelationID,
			command.CausationID,
			command.CreatedAt,
			opening,
			wallet.Balance(),
		)); err != nil {
			return err
		}
		if err := tx.AppendOutboxEvent(ctx, newWalletBalanceChangedEvent(
			command.BalanceChangedEventID,
			command.CorrelationID,
			command.CausationID,
			command.CreatedAt,
			entry,
			wallet.Version(),
		)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return OpenWalletResult{}, err
	}

	return OpenWalletResult{
		WalletID: wallet.ID(),
		PlayerID: wallet.PlayerID(),
		Balance:  wallet.Balance(),
		Version:  wallet.Version(),
	}, nil
}
