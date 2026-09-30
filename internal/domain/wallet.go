package domain

import (
	"errors"
	"math"
	"time"
)

var (
	ErrInvalidWalletID       = errors.New("invalid wallet ID")
	ErrInvalidPlayerID       = errors.New("invalid player ID")
	ErrNegativeBalance       = errors.New("wallet balance cannot be negative")
	ErrInvalidTimestamp      = errors.New("invalid timestamp")
	ErrNonPositiveAmount     = errors.New("amount must be greater than zero")
	ErrStaleTimestamp        = errors.New("timestamp cannot be earlier than wallet update")
	ErrInsufficientFunds     = errors.New("insufficient funds")
	ErrInvalidWalletVersion  = errors.New("wallet version must be greater than zero")
	ErrWalletVersionOverflow = errors.New("wallet version overflows int64")
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
	if err := validateWalletIdentityAndBalance(
		id,
		playerID,
		initialBalance,
	); err != nil {
		return Wallet{}, err
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

func RehydrateWallet(
	id string,
	playerID string,
	balance Money,
	version int64,
	createdAt time.Time,
	updatedAt time.Time,
) (Wallet, error) {
	if err := validateWalletIdentityAndBalance(
		id,
		playerID,
		balance,
	); err != nil {
		return Wallet{}, err
	}

	if version < 1 {
		return Wallet{}, ErrInvalidWalletVersion
	}

	if createdAt.IsZero() || updatedAt.IsZero() {
		return Wallet{}, ErrInvalidTimestamp
	}

	createdAt = createdAt.UTC()
	updatedAt = updatedAt.UTC()

	if updatedAt.Before(createdAt) {
		return Wallet{}, ErrStaleTimestamp
	}

	return Wallet{
		id:        id,
		playerID:  playerID,
		currency:  balance.currency,
		balance:   balance,
		version:   version,
		createdAt: createdAt.UTC(),
		updatedAt: updatedAt.UTC(),
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
	normalizedNow, err := w.validateTransitionTime(now)
	if err != nil {
		return err
	}

	if err := validatePositiveAmount(amount); err != nil {
		return err
	}

	newBalance, err := w.balance.Add(amount)
	if err != nil {
		return err
	}

	if err := validatePositiveAmount(amount); err != nil {
		return err
	}

	if err := w.validateVersionIncrement(); err != nil {
		return err
	}

	w.balance = newBalance
	w.version++
	w.updatedAt = normalizedNow

	return nil
}

func (w *Wallet) Debit(amount Money, now time.Time) error {
	normalizedNow, err := w.validateTransitionTime(now)
	if err != nil {
		return err
	}

	if err := validatePositiveAmount(amount); err != nil {
		return err
	}

	if err := w.validateVersionIncrement(); err != nil {
		return err
	}

	comparison, err := amount.Compare(w.balance)
	if err != nil {
		return err
	}

	if comparison > 0 {
		return ErrInsufficientFunds
	}

	newBalance, err := w.balance.Subtract(amount)
	if err != nil {
		return err
	}

	w.balance = newBalance
	w.version++
	w.updatedAt = normalizedNow

	return nil
}

func validatePositiveAmount(amount Money) error {
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

	return nil
}

func (w Wallet) validateTransitionTime(now time.Time) (time.Time, error) {
	if now.IsZero() {
		return time.Time{}, ErrInvalidTimestamp
	}

	now = now.UTC()

	if now.Before(w.updatedAt) {
		return time.Time{}, ErrStaleTimestamp
	}

	return now, nil
}

func validateWalletIdentityAndBalance(
	id string,
	playerID string,
	balance Money,
) error {
	if id == "" {
		return ErrInvalidWalletID
	}

	if playerID == "" {
		return ErrInvalidPlayerID
	}

	if err := balance.validate(); err != nil {
		return err
	}

	zero, err := ZeroMoney(balance.Currency())
	if err != nil {
		return err
	}

	comparison, err := balance.Compare(zero)
	if err != nil {
		return err
	}

	if comparison < 0 {
		return ErrNegativeBalance
	}

	return nil
}

func (w Wallet) validateVersionIncrement() error {
	if w.version == math.MaxInt64 {
		return ErrWalletVersionOverflow
	}

	return nil
}
