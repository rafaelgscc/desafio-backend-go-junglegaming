package domain

import (
	"fmt"
	"math"
	"strconv"
)

type Money struct {
	amountInCents int64
	currency      string
}

func NewMoney(amount, currency string) (Money, error) {
	// Validate the amount format (e.g "25.00")
	if len(amount) < 4 || amount[len(amount)-3] != '.' {
		return Money{}, fmt.Errorf("invalid money amount: %q", amount)
	}

	integerPart := amount[:len(amount)-3]
	fractionalPart := amount[len(amount)-2:]

	integer, err := strconv.ParseInt(integerPart, 10, 64)
	if err != nil {
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
