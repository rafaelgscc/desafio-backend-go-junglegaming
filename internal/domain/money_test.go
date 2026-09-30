package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNewMoneyCreatesValidBRLAmount(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error: %v", err)
	}

	if got, want := money.Amount(), "25.00"; got != want {
		t.Errorf("Money.Amount() = %q, want %q", got, want)
	}

	if got, want := money.Currency(), "BRL"; got != want {
		t.Errorf("Money.Currency() = %q, want %q", got, want)
	}
}

func TestMoneyRegectsNegativeAmount(t *testing.T) {
	t.Parallel()

	_, err := NewMoney("-25.00", "BRL")
	if err == nil {
		t.Fatal("NewMoney() expected error for negative amount, got nil")
	}
}

func TestNewMoneyRejectsEmptyCurrency(t *testing.T) {
	t.Parallel()

	_, err := NewMoney("25.00", "")
	if err == nil {
		t.Fatal("NewMoney() expected error for empty currency, got nil")
	}
}

func TestNewMoneyRejectsInvalidCurrencyFormat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		currency string
	}{
		{name: "lowercase", currency: "brl"},
		{name: "fewer than three letters", currency: "BR"},
		{name: "more than three letters", currency: "BRLL"},
		{name: "numeric", currency: "123"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewMoney("25.00", testCase.currency)
			if err == nil {
				t.Fatalf("NewMoney() expected error for currency %q, got nil", testCase.currency)
			}
		})
	}
}

func TestNewMoneyRejectsInvalidAmountFormat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		amount string
	}{
		{name: "empty", amount: ""},
		{name: "without decimal places", amount: "25"},
		{name: "one decimal place", amount: "25.0"},
		{name: "more than two decimal places", amount: "25.000"},
		{name: "NaN", amount: "NaN"},
		{name: "positive infinity", amount: "Infinity"},
		{name: "scientific notation", amount: "2.5e1"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewMoney(testCase.amount, "BRL")
			if err == nil {
				t.Fatalf("NewMoney() expected error for amount %q, got nil", testCase.amount)
			}
		})
	}
}

func TestMoneyMarshalsToExternalContract(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error: %v", err)
	}

	got, err := json.Marshal(money)
	if err != nil {
		t.Fatalf("json.Marshal() unexpected error: %v", err)
	}

	want := `{"amount":"25.00","currency":"BRL"}`
	if string(got) != want {
		t.Errorf("json.Marshal() = %s, want %s", got, want)
	}
}

func TestMoneyAddsAmountsWithSameCurrency(t *testing.T) {
	t.Parallel()

	left, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for left operand: %v", err)
	}

	right, err := NewMoney("10.50", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for right operand: %v", err)
	}

	result, err := left.Add(right)
	if err != nil {
		t.Fatalf("Money.Add() unexpected error: %v", err)
	}

	if got, want := result.Amount(), "35.50"; got != want {
		t.Errorf("Money.Add().Amount() = %q, want %q", got, want)
	}

	if got, want := result.Currency(), "BRL"; got != want {
		t.Errorf("Money.Add().Currency() = %q, want %q", got, want)
	}

	if got, want := left.Amount(), "25.00"; got != want {
		t.Errorf("left operand changed to %q, want %q", got, want)
	}

	if got, want := right.Amount(), "10.50"; got != want {
		t.Errorf("right operand changed to %q, want %q", got, want)
	}
}

func TestMoneyAddRejectsDifferentCurrencies(t *testing.T) {
	t.Parallel()

	brl, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for BRL: %v", err)
	}

	usd, err := NewMoney("10.00", "USD")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for USD: %v", err)
	}

	_, err = brl.Add(usd)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("Money.Add() error = %v, want ErrCurrencyMismatch", err)
	}
}

func TestMoneyAddRejectsOverflow(t *testing.T) {
	t.Parallel()

	maximum, err := NewMoney("92233720368547758.07", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for maximum amount: %v", err)
	}

	oneCent, err := NewMoney("0.01", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for one cent: %v", err)
	}

	_, err = maximum.Add(oneCent)
	if !errors.Is(err, ErrMoneyOverflow) {
		t.Fatalf("Money.Add() error = %v, want ErrMoneyOverflow", err)
	}
}

func TestMoneySubtractsAmountsWithSameCurrency(t *testing.T) {
	t.Parallel()

	minuend, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for minuend: %v", err)
	}

	subtrahend, err := NewMoney("10.50", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for subtrahend: %v", err)
	}

	result, err := minuend.Subtract(subtrahend)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected error: %v", err)
	}

	if got, want := result.Amount(), "14.50"; got != want {
		t.Errorf("Money.Subtract().Amount() = %q, want %q", got, want)
	}

	if got, want := result.Currency(), "BRL"; got != want {
		t.Errorf("Money.Subtract().Currency() = %q, want %q", got, want)
	}
}
