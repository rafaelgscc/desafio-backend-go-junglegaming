package domain

import "testing"

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
