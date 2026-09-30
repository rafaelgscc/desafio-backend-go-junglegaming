package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
)

type Money struct {
	amountInCents int64
	currency      string
}

var (
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrMoneyOverflow    = errors.New("money amount overflows int64")
)

func NewMoney(amount, currency string) (Money, error) {
	// Validate the amount format (e.g "25.00")
	if len(amount) < 4 || amount[len(amount)-3] != '.' {
		return Money{}, fmt.Errorf("invalid money amount: %q", amount)
	}

	if !isValidCurrency(currency) {
		return Money{}, fmt.Errorf("invalid currency format %q", currency)
	}

	integerPart := amount[:len(amount)-3]
	fractionalPart := amount[len(amount)-2:]

	//convert the integer and factional parts to int64 and check for negative values
	integer, err := strconv.ParseInt(integerPart, 10, 64)
	if err != nil || integer < 0 {
		return Money{}, fmt.Errorf("invalid money amount: %q", amount)
	}

	fraction, err := strconv.ParseInt(fractionalPart, 10, 64)
	if err != nil || fraction < 0 || fraction > 99 {
		return Money{}, fmt.Errorf("invalid money amount: %q", amount)
	}

	if integer > (math.MaxInt64-fraction)/100 {
		return Money{}, fmt.Errorf("money amount overflows int64: %q", amount)
	}

	return Money{
		amountInCents: integer*100 + fraction,
		currency:      currency,
	}, nil
}

func (m Money) Amount() string {
	return fmt.Sprintf("%d.%02d", m.amountInCents/100, m.amountInCents%100)
}

func (m Money) Currency() string {
	return m.currency
}

// Add adds two Money values with the same currency and returns a new Money value or an error.
func isValidCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}

	for i := 0; i < len(currency); i++ {
		if currency[i] < 'A' || currency[i] > 'Z' {
			return false
		}
	}
	return true
}

// implement the json interface for Money
func (m Money) MarshalJSON() ([]byte, error) {
	type moneyJSON struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	return json.Marshal(moneyJSON{
		Amount:   m.Amount(),
		Currency: m.Currency(),
	})
}

// add function to add two money values
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf(
			"%w: %s and %s",
			ErrCurrencyMismatch,
			m.currency,
			other.currency,
		)
	}

	if other.amountInCents > 0 &&
		m.amountInCents > math.MaxInt64-other.amountInCents {
		return Money{}, ErrMoneyOverflow
	}

	if other.amountInCents < 0 &&
		m.amountInCents < math.MinInt64-other.amountInCents {
		return Money{}, ErrMoneyOverflow
	}

	return Money{
		amountInCents: m.amountInCents + other.amountInCents,
		currency:      m.currency,
	}, nil
}

func (m Money) Subtract(other Money) (Money, error) {
	return Money{
		amountInCents: m.amountInCents - other.amountInCents,
		currency:      m.currency,
	}, nil
}
