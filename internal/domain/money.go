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
	ErrInvalidAmount    = errors.New("invalid money amount")
	ErrInvalidCurrency  = errors.New("invalid currency")
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrMoneyOverflow    = errors.New("money amount overflows int64")
)

func NewMoney(amount, currency string) (Money, error) {
	// Validate the amount format (e.g "25.00")
	if len(amount) < 4 || amount[len(amount)-3] != '.' {
		return Money{}, invalidAmountError(amount)
	}

	if amount[0] == '-' {
		return Money{}, invalidAmountError(amount)
	}

	if !isValidCurrency(currency) {
		return Money{}, fmt.Errorf(
			"%w: %q",
			ErrInvalidCurrency,
			currency,
		)
	}

	integerPart := amount[:len(amount)-3]
	fractionalPart := amount[len(amount)-2:]

	//convert the integer and factional parts to int64 and check for negative values
	integer, err := strconv.ParseInt(integerPart, 10, 64)
	if errors.Is(err, strconv.ErrRange) {
		return Money{}, fmt.Errorf(
			"%w: %q",
			ErrMoneyOverflow,
			amount,
		)
	}

	if err != nil || integer < 0 {
		return Money{}, invalidAmountError(amount)
	}

	fraction, err := strconv.ParseInt(fractionalPart, 10, 64)
	if err != nil || fraction < 0 || fraction > 99 {
		return Money{}, invalidAmountError(amount)
	}

	if integer > (math.MaxInt64-fraction)/100 {
		return Money{}, fmt.Errorf(
			"%w: %q",
			ErrMoneyOverflow,
			amount,
		)
	}

	return Money{
		amountInCents: integer*100 + fraction,
		currency:      currency,
	}, nil
}

func (m Money) Amount() string {
	sign := ""
	var magnitude uint64

	if m.amountInCents < 0 {
		sign = "-"
		//the expression can works for math.MinInt64, that can't be negated directly as int64.
		magnitude = uint64(-(m.amountInCents + 1)) + 1
	} else {
		magnitude = uint64(m.amountInCents)
	}

	return fmt.Sprintf(
		"%s%d.%02d",
		sign,
		magnitude/100,
		magnitude%100,
	)
}

func (m Money) Currency() string {
	return m.currency
}

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

// implement the json serialization of Money
func (m Money) MarshalJSON() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}

	type moneyJSON struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}

	return json.Marshal(moneyJSON{
		Amount:   m.Amount(),
		Currency: m.Currency(),
	})
}

// implements the deserialization of Money from JSON
func (m *Money) UnmarshalJSON(data []byte) error {
	type moneyJSON struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}

	var payload moneyJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode money JSON: %w", err)
	}

	money, err := NewMoney(payload.Amount, payload.Currency)
	if err != nil {
		return err
	}

	*m = money
	return nil
}

// add function to add two money values
func (m Money) Add(other Money) (Money, error) {
	if err := m.ensureSameCurrency(other); err != nil {
		return Money{}, err
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

// Subtract funtion to subtract two money values
func (m Money) Subtract(other Money) (Money, error) {
	if err := m.ensureSameCurrency(other); err != nil {
		return Money{}, err
	}

	if other.amountInCents > 0 &&
		m.amountInCents < math.MinInt64+other.amountInCents {
		return Money{}, ErrMoneyOverflow
	}

	if other.amountInCents < 0 &&
		m.amountInCents > math.MaxInt64+other.amountInCents {
		return Money{}, ErrMoneyOverflow
	}

	return Money{
		amountInCents: m.amountInCents - other.amountInCents,
		currency:      m.currency,
	}, nil
}

func (m Money) Negate() (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}

	if m.amountInCents == math.MinInt64 {
		return Money{}, ErrMoneyOverflow
	}

	return Money{
		amountInCents: -m.amountInCents,
		currency:      m.currency,
	}, nil
}

func (m Money) Compare(other Money) (int, error) {
	if err := m.ensureSameCurrency(other); err != nil {
		return 0, err
	}

	switch {
	case m.amountInCents < other.amountInCents:
		return -1, nil
	case m.amountInCents > other.amountInCents:
		return 1, nil
	default:
		return 0, nil
	}
}

func (m Money) validate() error {
	if !isValidCurrency(m.currency) {
		return fmt.Errorf(
			"%w: %q",
			ErrInvalidCurrency,
			m.currency,
		)
	}

	return nil
}

func (m Money) ensureSameCurrency(other Money) error {
	if err := m.validate(); err != nil {
		return err
	}

	if err := other.validate(); err != nil {
		return err
	}

	if m.currency == other.currency {
		return nil
	}

	return fmt.Errorf(
		"%w: %s and %s",
		ErrCurrencyMismatch,
		m.currency,
		other.currency,
	)
}

func ZeroMoney(currency string) (Money, error) {
	return NewMoney("0.00", currency)
}

func invalidAmountError(amount string) error {
	return fmt.Errorf("%w: %q", ErrInvalidAmount, amount)
}
