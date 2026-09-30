package domain

import (
	"errors"
	"fmt"
	"math"
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

func TestNewWalletRejectsUninitializedBalance(t *testing.T) {
	t.Parallel()

	var initialBalance Money
	now := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)

	_, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		initialBalance,
		now,
	)
	if !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("NewWallet() error = %v, want ErrInvalidCurrency", err)
	}
}

func TestNewWalletRejectsNegativeInitialBalance(t *testing.T) {
	t.Parallel()

	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for zero: %v", err)
	}

	oneCent, err := NewMoney("0.01", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for one cent: %v", err)
	}

	negativeBalance, err := zero.Subtract(oneCent)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected setup error: %v", err)
	}

	now := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	_, err = NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		negativeBalance,
		now,
	)
	if !errors.Is(err, ErrNegativeBalance) {
		t.Fatalf("NewWallet() error = %v, want ErrNegativeBalance", err)
	}
}

func TestNewWalletRejectsZeroTimestamp(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	_, err = NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		initialBalance,
		time.Time{},
	)
	if !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("NewWallet() error = %v, want ErrInvalidTimestamp", err)
	}
}

func TestNewWalletNormalizesTimestampsToUTC(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	localTime := time.Date(
		2026,
		time.September,
		30,
		9,
		0,
		0,
		0,
		time.FixedZone("BRT", -3*60*60),
	)
	want := localTime.UTC()

	wallet, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		initialBalance,
		localTime,
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected error: %v", err)
	}

	if got := wallet.CreatedAt(); !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("Wallet.CreatedAt() = %v (%v), want %v (UTC)", got, got.Location(), want)
	}

	if got := wallet.UpdatedAt(); !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("Wallet.UpdatedAt() = %v (%v), want %v (UTC)", got, got.Location(), want)
	}
}

func TestWalletCreditUpdatesBalanceVersionAndTimestamp(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for initial balance: %v", err)
	}

	credit, err := NewMoney("25.50", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for credit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	wallet, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		initialBalance,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}

	updatedAt := time.Date(
		2026,
		time.September,
		30,
		10,
		30,
		0,
		0,
		time.FixedZone("BRT", -3*60*60),
	)

	if err := wallet.Credit(credit, updatedAt); err != nil {
		t.Fatalf("Wallet.Credit() unexpected error: %v", err)
	}

	if got, want := wallet.Balance().Amount(), "125.50"; got != want {
		t.Errorf("Wallet.Balance().Amount() = %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(2); got != want {
		t.Errorf("Wallet.Version() = %d, want %d", got, want)
	}

	if got := wallet.CreatedAt(); !got.Equal(createdAt) || got.Location() != time.UTC {
		t.Errorf("Wallet.CreatedAt() = %v, want unchanged %v", got, createdAt)
	}

	if got, want := wallet.UpdatedAt(), updatedAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("Wallet.UpdatedAt() = %v (%v), want %v (UTC)", got, got.Location(), want)
	}

	if got, want := credit.Amount(), "25.50"; got != want {
		t.Errorf("credit Money changed to %q, want %q", got, want)
	}
}

func TestWalletCreditRejectsNonPositiveAmountWithoutChangingState(t *testing.T) {
	t.Parallel()

	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for zero: %v", err)
	}

	oneCent, err := NewMoney("0.01", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for one cent: %v", err)
	}

	negative, err := zero.Subtract(oneCent)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected setup error: %v", err)
	}

	testCases := []struct {
		name   string
		amount Money
	}{
		{name: "zero", amount: zero},
		{name: "negative", amount: negative},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			initialBalance, err := NewMoney("100.00", "BRL")
			if err != nil {
				t.Fatalf("NewMoney() unexpected setup error: %v", err)
			}

			createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
			wallet, err := NewWallet(
				"0192f291-27dd-7d3f-8071-5f8685deef37",
				"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
				initialBalance,
				createdAt,
			)
			if err != nil {
				t.Fatalf("NewWallet() unexpected setup error: %v", err)
			}

			err = wallet.Credit(testCase.amount, createdAt.Add(time.Hour))
			if !errors.Is(err, ErrNonPositiveAmount) {
				t.Fatalf("Wallet.Credit() error = %v, want ErrNonPositiveAmount", err)
			}

			if got, want := wallet.Balance().Amount(), "100.00"; got != want {
				t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
			}

			if got, want := wallet.Version(), int64(1); got != want {
				t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
			}

			if got := wallet.UpdatedAt(); !got.Equal(createdAt) {
				t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, createdAt)
			}
		})
	}
}

func TestWalletCreditRejectsDifferentCurrencyWithoutChangingState(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for initial balance: %v", err)
	}

	credit, err := NewMoney("25.00", "USD")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for credit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	wallet, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		initialBalance,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}

	err = wallet.Credit(credit, createdAt.Add(time.Hour))
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("Wallet.Credit() error = %v, want ErrCurrencyMismatch", err)
	}

	if got, want := wallet.Balance().Amount(), "100.00"; got != want {
		t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(1); got != want {
		t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
	}

	if got := wallet.UpdatedAt(); !got.Equal(createdAt) {
		t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, createdAt)
	}
}

func TestWalletCreditRejectsZeroTimestampWithoutChangingState(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for initial balance: %v", err)
	}

	credit, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for credit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	wallet, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		initialBalance,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}

	err = wallet.Credit(credit, time.Time{})
	if !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("Wallet.Credit() error = %v, want ErrInvalidTimestamp", err)
	}

	if got, want := wallet.Balance().Amount(), "100.00"; got != want {
		t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(1); got != want {
		t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
	}

	if got := wallet.UpdatedAt(); !got.Equal(createdAt) {
		t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, createdAt)
	}
}

func TestWalletCreditRejectsStaleTimestampWithoutChangingState(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for initial balance: %v", err)
	}

	credit, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for credit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	wallet, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		initialBalance,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}

	err = wallet.Credit(credit, createdAt.Add(-time.Minute))
	if !errors.Is(err, ErrStaleTimestamp) {
		t.Fatalf("Wallet.Credit() error = %v, want ErrStaleTimestamp", err)
	}

	if got, want := wallet.Balance().Amount(), "100.00"; got != want {
		t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(1); got != want {
		t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
	}

	if got := wallet.UpdatedAt(); !got.Equal(createdAt) {
		t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, createdAt)
	}
}

func TestWalletDebitUpdatesBalanceVersionAndTimestamp(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for initial balance: %v", err)
	}

	debit, err := NewMoney("25.50", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for debit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	wallet, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		initialBalance,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}

	updatedAt := time.Date(
		2026,
		time.September,
		30,
		10,
		30,
		0,
		0,
		time.FixedZone("BRT", -3*60*60),
	)

	if err := wallet.Debit(debit, updatedAt); err != nil {
		t.Fatalf("Wallet.Debit() unexpected error: %v", err)
	}

	if got, want := wallet.Balance().Amount(), "74.50"; got != want {
		t.Errorf("Wallet.Balance().Amount() = %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(2); got != want {
		t.Errorf("Wallet.Version() = %d, want %d", got, want)
	}

	if got := wallet.CreatedAt(); !got.Equal(createdAt) || got.Location() != time.UTC {
		t.Errorf("Wallet.CreatedAt() = %v, want unchanged %v", got, createdAt)
	}

	if got, want := wallet.UpdatedAt(), updatedAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("Wallet.UpdatedAt() = %v (%v), want %v (UTC)", got, got.Location(), want)
	}

	if got, want := debit.Amount(), "25.50"; got != want {
		t.Errorf("debit Money changed to %q, want %q", got, want)
	}
}

func TestWalletDebitRejectsInsufficientFundsWithoutChangingState(t *testing.T) {
	t.Parallel()

	initialBalance, err := NewMoney("50.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for initial balance: %v", err)
	}

	debit, err := NewMoney("80.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for debit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	wallet, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		initialBalance,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}

	err = wallet.Debit(debit, createdAt.Add(time.Hour))
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("Wallet.Debit() error = %v, want ErrInsufficientFunds", err)
	}

	if got, want := wallet.Balance().Amount(), "50.00"; got != want {
		t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(1); got != want {
		t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
	}

	if got := wallet.UpdatedAt(); !got.Equal(createdAt) {
		t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, createdAt)
	}
}

func TestWalletDebitRejectsNonPositiveAmountWithoutChangingState(t *testing.T) {
	t.Parallel()

	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for zero: %v", err)
	}

	oneCent, err := NewMoney("0.01", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for one cent: %v", err)
	}

	negative, err := zero.Subtract(oneCent)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected setup error: %v", err)
	}

	testCases := []struct {
		name   string
		amount Money
	}{
		{name: "zero", amount: zero},
		{name: "negative", amount: negative},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			initialBalance, err := NewMoney("100.00", "BRL")
			if err != nil {
				t.Fatalf("NewMoney() unexpected setup error: %v", err)
			}

			createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
			wallet, err := NewWallet(
				"0192f291-27dd-7d3f-8071-5f8685deef37",
				"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
				initialBalance,
				createdAt,
			)
			if err != nil {
				t.Fatalf("NewWallet() unexpected setup error: %v", err)
			}

			err = wallet.Debit(testCase.amount, createdAt.Add(time.Hour))
			if !errors.Is(err, ErrNonPositiveAmount) {
				t.Fatalf("Wallet.Debit() error = %v, want ErrNonPositiveAmount", err)
			}

			if got, want := wallet.Balance().Amount(), "100.00"; got != want {
				t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
			}

			if got, want := wallet.Version(), int64(1); got != want {
				t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
			}

			if got := wallet.UpdatedAt(); !got.Equal(createdAt) {
				t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, createdAt)
			}
		})
	}
}

func TestWalletDebitRejectsInvalidTimestampWithoutChangingState(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	testCases := []struct {
		name    string
		now     time.Time
		wantErr error
	}{
		{name: "zero timestamp", now: time.Time{}, wantErr: ErrInvalidTimestamp},
		{name: "stale timestamp", now: createdAt.Add(-time.Minute), wantErr: ErrStaleTimestamp},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			initialBalance, err := NewMoney("100.00", "BRL")
			if err != nil {
				t.Fatalf("NewMoney() unexpected setup error for initial balance: %v", err)
			}

			debit, err := NewMoney("25.00", "BRL")
			if err != nil {
				t.Fatalf("NewMoney() unexpected setup error for debit: %v", err)
			}

			wallet, err := NewWallet(
				"0192f291-27dd-7d3f-8071-5f8685deef37",
				"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
				initialBalance,
				createdAt,
			)
			if err != nil {
				t.Fatalf("NewWallet() unexpected setup error: %v", err)
			}

			err = wallet.Debit(debit, testCase.now)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("Wallet.Debit() error = %v, want %v", err, testCase.wantErr)
			}

			if got, want := wallet.Balance().Amount(), "100.00"; got != want {
				t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
			}

			if got, want := wallet.Version(), int64(1); got != want {
				t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
			}

			if got := wallet.UpdatedAt(); !got.Equal(createdAt) {
				t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, createdAt)
			}
		})
	}
}

func TestRehydrateWalletRestoresPersistedState(t *testing.T) {
	t.Parallel()

	balance, err := NewMoney("275.35", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	location := time.FixedZone("BRT", -3*60*60)
	createdAt := time.Date(2026, time.September, 29, 9, 0, 0, 0, location)
	updatedAt := time.Date(2026, time.September, 30, 10, 30, 0, 0, location)

	wallet, err := RehydrateWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		balance,
		7,
		createdAt,
		updatedAt,
	)
	if err != nil {
		t.Fatalf("RehydrateWallet() unexpected error: %v", err)
	}

	if got, want := wallet.ID(), "0192f291-27dd-7d3f-8071-5f8685deef37"; got != want {
		t.Errorf("Wallet.ID() = %q, want %q", got, want)
	}

	if got, want := wallet.PlayerID(), "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1"; got != want {
		t.Errorf("Wallet.PlayerID() = %q, want %q", got, want)
	}

	if got, want := wallet.Balance().Amount(), "275.35"; got != want {
		t.Errorf("Wallet.Balance().Amount() = %q, want %q", got, want)
	}

	if got, want := wallet.Currency(), "BRL"; got != want {
		t.Errorf("Wallet.Currency() = %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(7); got != want {
		t.Errorf("Wallet.Version() = %d, want %d", got, want)
	}

	if got, want := wallet.CreatedAt(), createdAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("Wallet.CreatedAt() = %v (%v), want %v (UTC)", got, got.Location(), want)
	}

	if got, want := wallet.UpdatedAt(), updatedAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("Wallet.UpdatedAt() = %v (%v), want %v (UTC)", got, got.Location(), want)
	}
}

func TestRehydrateWalletRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	balance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	createdAt := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	for _, version := range []int64{0, -1} {
		version := version
		t.Run(fmt.Sprintf("version_%d", version), func(t *testing.T) {
			t.Parallel()

			_, err := RehydrateWallet(
				"0192f291-27dd-7d3f-8071-5f8685deef37",
				"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
				balance,
				version,
				createdAt,
				updatedAt,
			)
			if !errors.Is(err, ErrInvalidWalletVersion) {
				t.Fatalf("RehydrateWallet() error = %v, want ErrInvalidWalletVersion", err)
			}
		})
	}
}

func TestRehydrateWalletRejectsInvalidTimestamps(t *testing.T) {
	t.Parallel()

	balance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	validCreatedAt := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	validUpdatedAt := validCreatedAt.Add(time.Hour)

	testCases := []struct {
		name      string
		createdAt time.Time
		updatedAt time.Time
		wantErr   error
	}{
		{
			name:      "zero creation timestamp",
			createdAt: time.Time{},
			updatedAt: validUpdatedAt,
			wantErr:   ErrInvalidTimestamp,
		},
		{
			name:      "zero update timestamp",
			createdAt: validCreatedAt,
			updatedAt: time.Time{},
			wantErr:   ErrInvalidTimestamp,
		},
		{
			name:      "update before creation",
			createdAt: validCreatedAt,
			updatedAt: validCreatedAt.Add(-time.Minute),
			wantErr:   ErrStaleTimestamp,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := RehydrateWallet(
				"0192f291-27dd-7d3f-8071-5f8685deef37",
				"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
				balance,
				1,
				testCase.createdAt,
				testCase.updatedAt,
			)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("RehydrateWallet() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestRehydrateWalletRejectsInvalidIdentityAndBalance(t *testing.T) {
	t.Parallel()

	validBalance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for valid balance: %v", err)
	}

	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for zero: %v", err)
	}

	oneCent, err := NewMoney("0.01", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for one cent: %v", err)
	}

	negativeBalance, err := zero.Subtract(oneCent)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected setup error: %v", err)
	}

	var uninitializedBalance Money
	validWalletID := "0192f291-27dd-7d3f-8071-5f8685deef37"
	validPlayerID := "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1"

	testCases := []struct {
		name     string
		walletID string
		playerID string
		balance  Money
		wantErr  error
	}{
		{
			name:     "empty wallet ID",
			walletID: "",
			playerID: validPlayerID,
			balance:  validBalance,
			wantErr:  ErrInvalidWalletID,
		},
		{
			name:     "empty player ID",
			walletID: validWalletID,
			playerID: "",
			balance:  validBalance,
			wantErr:  ErrInvalidPlayerID,
		},
		{
			name:     "uninitialized balance",
			walletID: validWalletID,
			playerID: validPlayerID,
			balance:  uninitializedBalance,
			wantErr:  ErrInvalidCurrency,
		},
		{
			name:     "negative balance",
			walletID: validWalletID,
			playerID: validPlayerID,
			balance:  negativeBalance,
			wantErr:  ErrNegativeBalance,
		},
	}

	createdAt := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := RehydrateWallet(
				testCase.walletID,
				testCase.playerID,
				testCase.balance,
				1,
				createdAt,
				updatedAt,
			)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("RehydrateWallet() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestWalletCreditRejectsVersionOverflowWithoutChangingState(t *testing.T) {
	t.Parallel()

	balance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance: %v", err)
	}

	credit, err := NewMoney("1.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for credit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	wallet, err := RehydrateWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		balance,
		math.MaxInt64,
		createdAt,
		updatedAt,
	)
	if err != nil {
		t.Fatalf("RehydrateWallet() unexpected setup error: %v", err)
	}

	err = wallet.Credit(credit, updatedAt.Add(time.Hour))
	if !errors.Is(err, ErrWalletVersionOverflow) {
		t.Fatalf("Wallet.Credit() error = %v, want ErrWalletVersionOverflow", err)
	}

	if got, want := wallet.Balance().Amount(), "100.00"; got != want {
		t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(math.MaxInt64); got != want {
		t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
	}

	if got := wallet.UpdatedAt(); !got.Equal(updatedAt) {
		t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, updatedAt)
	}
}

func TestWalletDebitRejectsVersionOverflowWithoutChangingState(t *testing.T) {
	t.Parallel()

	balance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance: %v", err)
	}

	debit, err := NewMoney("1.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for debit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	wallet, err := RehydrateWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		balance,
		math.MaxInt64,
		createdAt,
		updatedAt,
	)
	if err != nil {
		t.Fatalf("RehydrateWallet() unexpected setup error: %v", err)
	}

	err = wallet.Debit(debit, updatedAt.Add(time.Hour))
	if !errors.Is(err, ErrWalletVersionOverflow) {
		t.Fatalf("Wallet.Debit() error = %v, want ErrWalletVersionOverflow", err)
	}

	if got, want := wallet.Balance().Amount(), "100.00"; got != want {
		t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(math.MaxInt64); got != want {
		t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
	}

	if got := wallet.UpdatedAt(); !got.Equal(updatedAt) {
		t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, updatedAt)
	}
}

func TestWalletDebitAllowsExactBalance(t *testing.T) {
	t.Parallel()

	balance, err := NewMoney("10.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance: %v", err)
	}

	debit, err := NewMoney("10.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for debit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	wallet, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		balance,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}

	if err := wallet.Debit(debit, createdAt.Add(time.Hour)); err != nil {
		t.Fatalf("Wallet.Debit() unexpected error: %v", err)
	}

	if got, want := wallet.Balance().Amount(), "0.00"; got != want {
		t.Errorf("Wallet.Balance().Amount() = %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(2); got != want {
		t.Errorf("Wallet.Version() = %d, want %d", got, want)
	}
}

func TestWalletDebitRejectsDifferentCurrencyWithoutChangingState(t *testing.T) {
	t.Parallel()

	balance, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance: %v", err)
	}

	debit, err := NewMoney("25.00", "USD")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for debit: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	wallet, err := NewWallet(
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		balance,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWallet() unexpected setup error: %v", err)
	}

	err = wallet.Debit(debit, createdAt.Add(time.Hour))
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("Wallet.Debit() error = %v, want ErrCurrencyMismatch", err)
	}

	if got, want := wallet.Balance().Amount(), "100.00"; got != want {
		t.Errorf("Wallet.Balance().Amount() changed to %q, want %q", got, want)
	}

	if got, want := wallet.Version(), int64(1); got != want {
		t.Errorf("Wallet.Version() changed to %d, want %d", got, want)
	}

	if got := wallet.UpdatedAt(); !got.Equal(createdAt) {
		t.Errorf("Wallet.UpdatedAt() changed to %v, want %v", got, createdAt)
	}
}
