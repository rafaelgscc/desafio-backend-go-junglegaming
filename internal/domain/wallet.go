package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidWalletID = errors.New("invalid wallet ID")
	ErrInvalidPlayerID = errors.New("invalid player ID")
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
