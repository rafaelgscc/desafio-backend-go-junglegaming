package domain

import (
	"errors"
	"time"
)

var (
	ErrInconsistentLedgerBalance = errors.New("inconsistent ledger balance")
	ErrInvalidLedgerDirection    = errors.New("invalid ledger direction")
)

type LedgerDirection string

const (
	LedgerDirectionCredit LedgerDirection = "CREDIT"
	LedgerDirectionDebit  LedgerDirection = "DEBIT"
)

type WalletLedgerEntry struct {
	id            string
	walletID      string
	transactionID string
	direction     LedgerDirection
	money         Money
	balanceBefore Money
	balanceAfter  Money
	createdAt     time.Time
}

func NewWalletLedgerEntry(
	id string,
	walletID string,
	transactionID string,
	direction LedgerDirection,
	money Money,
	balanceBefore Money,
	balanceAfter Money,
	createdAt time.Time,
) (WalletLedgerEntry, error) {
	if direction == LedgerDirectionCredit {
		expectedBalance, err := balanceBefore.Add(money)
		if err != nil {
			return WalletLedgerEntry{}, err
		}

		comparison, err := expectedBalance.Compare(balanceAfter)
		if err != nil {
			return WalletLedgerEntry{}, err
		}

		if comparison != 0 {
			return WalletLedgerEntry{}, ErrInconsistentLedgerBalance
		}
	}

	var (
		expectedBalance Money
		err             error
	)

	switch direction {
	case LedgerDirectionCredit:
		expectedBalance, err = balanceBefore.Add(money)
	case LedgerDirectionDebit:
		expectedBalance, err = balanceBefore.Subtract(money)
	default:
		return WalletLedgerEntry{}, ErrInvalidLedgerDirection
	}

	if err != nil {
		return WalletLedgerEntry{}, err
	}

	comparison, err := expectedBalance.Compare(balanceAfter)
	if err != nil {
		return WalletLedgerEntry{}, err
	}

	if comparison != 0 {
		return WalletLedgerEntry{}, ErrInconsistentLedgerBalance
	}

	return WalletLedgerEntry{
		id:            id,
		walletID:      walletID,
		transactionID: transactionID,
		direction:     direction,
		money:         money,
		balanceBefore: balanceBefore,
		balanceAfter:  balanceAfter,
		createdAt:     createdAt.UTC(),
	}, nil
}

func (e WalletLedgerEntry) ID() string {
	return e.id
}

func (e WalletLedgerEntry) WalletID() string {
	return e.walletID
}

func (e WalletLedgerEntry) TransactionID() string {
	return e.transactionID
}

func (e WalletLedgerEntry) Direction() LedgerDirection {
	return e.direction
}

func (e WalletLedgerEntry) Money() Money {
	return e.money
}

func (e WalletLedgerEntry) BalanceBefore() Money {
	return e.balanceBefore
}

func (e WalletLedgerEntry) BalanceAfter() Money {
	return e.balanceAfter
}

func (e WalletLedgerEntry) CreatedAt() time.Time {
	return e.createdAt
}
