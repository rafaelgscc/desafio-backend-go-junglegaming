package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidWalletID   = errors.New("invalid wallet ID")
	ErrInvalidPlayerID   = errors.New("invalid player ID")
	ErrNegativeBalance   = errors.New("wallet balance cannot be negative")
	ErrInvalidTimestamp  = errors.New("invalid timestamp")
	ErrNonPositiveAmount = errors.New("amount must be greater than zero")
)

type Wallet struct {
	id        string
	playerID  string
	currency  string
	balance   Money
	version   int64
	createdAt time.Time
	updatedAt time.Time
}

func NewWallet(
	id string,
	playerID string,
	initialBalance Money,
	now time.Time,
) (Wallet, error) {
	if id == "" {
		return Wallet{}, ErrInvalidWalletID
	}

	if playerID == "" {
		return Wallet{}, ErrInvalidPlayerID
	}

	if err := initialBalance.validate(); err != nil {
		return Wallet{}, err
	}

	zero, err := ZeroMoney(initialBalance.Currency())
	if err != nil {
		return Wallet{}, err
	}

	comparison, err := initialBalance.Compare(zero)
	if err != nil {
		return Wallet{}, err
	}

	if comparison < 0 {
		return Wallet{}, ErrNegativeBalance
	}

	if now.IsZero() {
		return Wallet{}, ErrInvalidTimestamp
	}

	now = now.UTC()

	return Wallet{
		id:        id,
		playerID:  playerID,
		currency:  initialBalance.Currency(),
		balance:   initialBalance,
		version:   1,
		createdAt: now,
		updatedAt: now,
	}, nil
}

func (w Wallet) ID() string {
	return w.id
}

func (w Wallet) PlayerID() string {
	return w.playerID
}

func (w Wallet) Currency() string {
	return w.currency
}

func (w Wallet) Balance() Money {
	return w.balance
}

func (w Wallet) Version() int64 {
	return w.version
}

func (w Wallet) CreatedAt() time.Time {
	return w.createdAt
}

func (w Wallet) UpdatedAt() time.Time {
	return w.updatedAt
}

func (w *Wallet) Credit(amount Money, now time.Time) error {
	if now.IsZero() {
		return ErrInvalidTimestamp
	}

	if err := amount.validate(); err != nil {
		return err
	}

	zero, err := ZeroMoney(amount.Currency())
	if err != nil {
		return err
	}

	comparison, err := amount.Compare(zero)
	if err != nil {
		return err
	}

	if comparison <= 0 {
		return ErrNonPositiveAmount
	}

	newBalance, err := w.balance.Add(amount)
	if err != nil {
		return err
	}

	w.balance = newBalance
	w.version++
	w.updatedAt = now.UTC()

	return nil
}
