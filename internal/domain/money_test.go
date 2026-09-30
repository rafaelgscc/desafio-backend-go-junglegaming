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

func TestNewMoneyRejectsNegativeAmount(t *testing.T) {
	t.Parallel()

	for _, amount := range []string{"-25.00", "-0.00"} {
		amount := amount
		t.Run(amount, func(t *testing.T) {
			t.Parallel()

			_, err := NewMoney(amount, "BRL")
			if !errors.Is(err, ErrInvalidAmount) {
				t.Fatalf(
					"NewMoney() error for negative amount %q = %v, want ErrInvalidAmount",
					amount,
					err,
				)
			}
		})
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
			if !errors.Is(err, ErrInvalidAmount) {
				t.Fatalf(
					"NewMoney() error for amount %q = %v, want ErrInvalidAmount",
					testCase.amount,
					err,
				)
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

func TestMoneySubtractRejectsDifferentCurrencies(t *testing.T) {
	t.Parallel()

	brl, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for BRL: %v", err)
	}

	usd, err := NewMoney("10.00", "USD")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for USD: %v", err)
	}

	_, err = brl.Subtract(usd)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("Money.Subtract() error = %v, want ErrCurrencyMismatch", err)
	}
}

func TestMoneySubtractAllowsNegativeInternalResult(t *testing.T) {
	t.Parallel()

	minuend, err := NewMoney("10.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for minuend: %v", err)
	}

	subtrahend, err := NewMoney("25.50", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for subtrahend: %v", err)
	}

	result, err := minuend.Subtract(subtrahend)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected error: %v", err)
	}

	if got, want := result.Amount(), "-15.50"; got != want {
		t.Errorf("Money.Subtract().Amount() = %q, want %q", got, want)
	}
}

func TestMoneySubtractRejectsOverflow(t *testing.T) {
	t.Parallel()

	maximum, err := NewMoney("92233720368547758.07", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for maximum amount: %v", err)
	}

	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for zero: %v", err)
	}

	oneCent, err := NewMoney("0.01", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for one cent: %v", err)
	}

	twoCents, err := NewMoney("0.02", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for two cents: %v", err)
	}

	negativeMaximum, err := zero.Subtract(maximum)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected setup error: %v", err)
	}

	negativeOneCent, err := zero.Subtract(oneCent)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected setup error: %v", err)
	}

	testCases := []struct {
		name  string
		left  Money
		right Money
	}{
		{name: "above maximum", left: maximum, right: negativeOneCent},
		{name: "below minimum", left: negativeMaximum, right: twoCents},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := testCase.left.Subtract(testCase.right)
			if !errors.Is(err, ErrMoneyOverflow) {
				t.Fatalf("Money.Subtract() error = %v, want ErrMoneyOverflow", err)
			}
		})
	}
}

func TestMoneyNegatesAmount(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error: %v", err)
	}

	negated, err := money.Negate()
	if err != nil {
		t.Fatalf("Money.Negate() unexpected error: %v", err)
	}

	if got, want := negated.Amount(), "-25.00"; got != want {
		t.Errorf("Money.Negate().Amount() = %q, want %q", got, want)
	}

	if got, want := negated.Currency(), "BRL"; got != want {
		t.Errorf("Money.Negate().Currency() = %q, want %q", got, want)
	}

	if got, want := money.Amount(), "25.00"; got != want {
		t.Errorf("original Money changed to %q, want %q", got, want)
	}
}

func TestMoneyNegateRejectsMinimumInt64(t *testing.T) {
	t.Parallel()

	maximum, err := NewMoney("92233720368547758.07", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for maximum amount: %v", err)
	}

	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for zero: %v", err)
	}

	oneCent, err := NewMoney("0.01", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for one cent: %v", err)
	}

	negativeMaximum, err := zero.Subtract(maximum)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected setup error: %v", err)
	}

	minimum, err := negativeMaximum.Subtract(oneCent)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected setup error: %v", err)
	}

	_, err = minimum.Negate()
	if !errors.Is(err, ErrMoneyOverflow) {
		t.Fatalf("Money.Negate() error = %v, want ErrMoneyOverflow", err)
	}
}

func TestMoneyComparesAmountsWithSameCurrency(t *testing.T) {
	t.Parallel()

	ten, err := NewMoney("10.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for ten: %v", err)
	}

	twenty, err := NewMoney("20.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for twenty: %v", err)
	}

	testCases := []struct {
		name  string
		left  Money
		right Money
		want  int
	}{
		{name: "less than", left: ten, right: twenty, want: -1},
		{name: "equal", left: ten, right: ten, want: 0},
		{name: "greater than", left: twenty, right: ten, want: 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := testCase.left.Compare(testCase.right)
			if err != nil {
				t.Fatalf("Money.Compare() unexpected error: %v", err)
			}

			if got != testCase.want {
				t.Errorf("Money.Compare() = %d, want %d", got, testCase.want)
			}
		})
	}
}

func TestMoneyCompareRejectsDifferentCurrencies(t *testing.T) {
	t.Parallel()

	brl, err := NewMoney("10.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for BRL: %v", err)
	}

	usd, err := NewMoney("10.00", "USD")
	if err != nil {
		t.Fatalf("NewMoney() unexpected error for USD: %v", err)
	}

	_, err = brl.Compare(usd)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("Money.Compare() error = %v, want ErrCurrencyMismatch", err)
	}
}

func TestZeroMoneyCreatesZeroForCurrency(t *testing.T) {
	t.Parallel()

	money, err := ZeroMoney("BRL")
	if err != nil {
		t.Fatalf("ZeroMoney() unexpected error: %v", err)
	}

	if got, want := money.Amount(), "0.00"; got != want {
		t.Errorf("ZeroMoney().Amount() = %q, want %q", got, want)
	}

	if got, want := money.Currency(), "BRL"; got != want {
		t.Errorf("ZeroMoney().Currency() = %q, want %q", got, want)
	}
}

func TestZeroMoneyRejectsInvalidCurrency(t *testing.T) {
	t.Parallel()

	_, err := ZeroMoney("brl")
	if !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("ZeroMoney() error = %v, want ErrInvalidCurrency", err)
	}
}

func TestNewMoneyRejectsAmountOverflow(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		amount string
	}{
		{name: "one cent above maximum", amount: "92233720368547758.08"},
		{name: "integer part exceeds int64", amount: "999999999999999999999999999.00"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewMoney(testCase.amount, "BRL")
			if !errors.Is(err, ErrMoneyOverflow) {
				t.Fatalf(
					"NewMoney() error for amount %q = %v, want ErrMoneyOverflow",
					testCase.amount,
					err,
				)
			}
		})
	}
}

func TestMoneyUnmarshalsFromExternalContract(t *testing.T) {
	t.Parallel()

	data := []byte(`{"amount":"25.00","currency":"BRL"}`)

	var money Money
	if err := json.Unmarshal(data, &money); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}

	if got, want := money.Amount(), "25.00"; got != want {
		t.Errorf("unmarshaled Money.Amount() = %q, want %q", got, want)
	}

	if got, want := money.Currency(), "BRL"; got != want {
		t.Errorf("unmarshaled Money.Currency() = %q, want %q", got, want)
	}
}

func TestMoneyUnmarshalRejectsInvalidDomainValuesWithoutChangingReceiver(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		data    string
		wantErr error
	}{
		{
			name:    "invalid amount",
			data:    `{"amount":"25.000","currency":"BRL"}`,
			wantErr: ErrInvalidAmount,
		},
		{
			name:    "invalid currency",
			data:    `{"amount":"25.00","currency":"brl"}`,
			wantErr: ErrInvalidCurrency,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			money, err := NewMoney("10.00", "BRL")
			if err != nil {
				t.Fatalf("NewMoney() unexpected setup error: %v", err)
			}

			err = json.Unmarshal([]byte(testCase.data), &money)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("json.Unmarshal() error = %v, want %v", err, testCase.wantErr)
			}

			if got, want := money.Amount(), "10.00"; got != want {
				t.Errorf("Money.Amount() changed to %q, want %q", got, want)
			}

			if got, want := money.Currency(), "BRL"; got != want {
				t.Errorf("Money.Currency() changed to %q, want %q", got, want)
			}
		})
	}
}

func TestMoneyMarshalRejectsUninitializedValue(t *testing.T) {
	t.Parallel()

	var money Money

	_, err := json.Marshal(money)
	if !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("json.Marshal() error = %v, want ErrInvalidCurrency", err)
	}
}

func TestMoneyOperationsRejectUninitializedValues(t *testing.T) {
	t.Parallel()

	valid, err := NewMoney("10.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	var uninitialized Money

	testCases := []struct {
		name      string
		operation func() error
	}{
		{
			name: "add uninitialized receiver",
			operation: func() error {
				_, err := uninitialized.Add(valid)
				return err
			},
		},
		{
			name: "add uninitialized argument",
			operation: func() error {
				_, err := valid.Add(uninitialized)
				return err
			},
		},
		{
			name: "subtract uninitialized receiver",
			operation: func() error {
				_, err := uninitialized.Subtract(valid)
				return err
			},
		},
		{
			name: "subtract uninitialized argument",
			operation: func() error {
				_, err := valid.Subtract(uninitialized)
				return err
			},
		},
		{
			name: "compare uninitialized receiver",
			operation: func() error {
				_, err := uninitialized.Compare(valid)
				return err
			},
		},
		{
			name: "compare uninitialized argument",
			operation: func() error {
				_, err := valid.Compare(uninitialized)
				return err
			},
		},
		{
			name: "negate uninitialized receiver",
			operation: func() error {
				_, err := uninitialized.Negate()
				return err
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if err := testCase.operation(); !errors.Is(err, ErrInvalidCurrency) {
				t.Fatalf("operation error = %v, want ErrInvalidCurrency", err)
			}
		})
	}
}

func FuzzNewMoneyNeverPanics(f *testing.F) {
	seeds := []string{
		"0.00",
		"25.00",
		"-0.00",
		"92233720368547758.07",
		"92233720368547758.08",
		"NaN",
		"Infinity",
		"2.5e1",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, amount string) {
		money, err := NewMoney(amount, "BRL")
		if err != nil {
			return
		}

		reparsed, err := NewMoney(money.Amount(), money.Currency())
		if err != nil {
			t.Fatalf("NewMoney() rejected its canonical output %q: %v", money.Amount(), err)
		}

		comparison, err := money.Compare(reparsed)
		if err != nil {
			t.Fatalf("Money.Compare() unexpected error: %v", err)
		}

		if comparison != 0 {
			t.Fatalf("round trip changed value: before %q, after %q", money.Amount(), reparsed.Amount())
		}
	})
}
