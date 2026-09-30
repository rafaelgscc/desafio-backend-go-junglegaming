package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewWalletCreatesWalletWithInitialState(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	now := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	walletID := "0192f291-27dd-7d3f-8071-5f8685deef37"
	playerID := "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1"

	wallet, err := NewWallet(walletID, playerID, initialBalance, now)
	if err != nil {
		t.Fatalf("NewWallet() unexpected error: %v", err)
	}

	if got := wallet.ID(); got != walletID {
		t.Errorf("Wallet.ID() = %q, want %q", got, walletID)
	}

	if got := wallet.PlayerID(); got != playerID {
		t.Errorf("Wallet.PlayerID() = %q, want %q", got, playerID)
	}

	if got, want := wallet.Balance().Amount(), "100.00"; got != want {
		t.Errorf("Wallet.Balance().Amount() = %q, want %q", got, want)
	}

	if got, want := wallet.Currency(), "BRL"; got != want {
		t.Errorf("Wallet.Currency() = %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(1); got != want {
		t.Errorf("Wallet.Version() = %d, want %d", got, want)
	}

	if got := wallet.CreatedAt(); !got.Equal(now) {
		t.Errorf("Wallet.CreatedAt() = %s, want %s", got, now)
	}

	if got := wallet.UpdatedAt(); !got.Equal(now) {
		t.Errorf("Wallet.UpdatedAt() = %s, want %s", got, now)
	}
}

func TestNewWalletRejectsEmptyIdentifiers(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	now := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name     string
		walletID string
		playerID string
		wantErr  error
	}{
		{
			name:     "empty wallet ID",
			walletID: "",
			playerID: "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
			wantErr:  ErrInvalidWalletID,
		},
		{
			name:     "empty player ID",
			walletID: "0192f291-27dd-7d3f-8071-5f8685deef37",
			playerID: "",
			wantErr:  ErrInvalidPlayerID,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewWallet(
				testCase.walletID,
				testCase.playerID,
				initialBalance,
				now,
			)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("NewWallet() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}
