package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewWalletLedgerEntryCreatesValidCredit(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for money: %v", err)
	}

	balanceBefore, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance before: %v", err)
	}

	balanceAfter, err := NewMoney("125.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance after: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	entryID := "0192f300-27dd-7d3f-8071-5f8685deef37"
	walletID := "0192f291-27dd-7d3f-8071-5f8685deef37"
	transactionID := "0192f298-345e-7e38-af88-e43f851a819d"

	entry, err := NewWalletLedgerEntry(
		entryID,
		walletID,
		transactionID,
		LedgerDirectionCredit,
		money,
		balanceBefore,
		balanceAfter,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewWalletLedgerEntry() unexpected error: %v", err)
	}

	if got := entry.ID(); got != entryID {
		t.Errorf("WalletLedgerEntry.ID() = %q, want %q", got, entryID)
	}

	if got := entry.WalletID(); got != walletID {
		t.Errorf("WalletLedgerEntry.WalletID() = %q, want %q", got, walletID)
	}

	if got := entry.TransactionID(); got != transactionID {
		t.Errorf("WalletLedgerEntry.TransactionID() = %q, want %q", got, transactionID)
	}

	if got := entry.Direction(); got != LedgerDirectionCredit {
		t.Errorf("WalletLedgerEntry.Direction() = %q, want %q", got, LedgerDirectionCredit)
	}

	if got, want := entry.Money().Amount(), "25.00"; got != want {
		t.Errorf("WalletLedgerEntry.Money().Amount() = %q, want %q", got, want)
	}

	if got, want := entry.BalanceBefore().Amount(), "100.00"; got != want {
		t.Errorf("WalletLedgerEntry.BalanceBefore().Amount() = %q, want %q", got, want)
	}

	if got, want := entry.BalanceAfter().Amount(), "125.00"; got != want {
		t.Errorf("WalletLedgerEntry.BalanceAfter().Amount() = %q, want %q", got, want)
	}

	if got := entry.CreatedAt(); !got.Equal(createdAt) || got.Location() != time.UTC {
		t.Errorf("WalletLedgerEntry.CreatedAt() = %v (%v), want %v (UTC)", got, got.Location(), createdAt)
	}
}

func TestNewWalletLedgerEntryRejectsInconsistentCreditBalance(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for money: %v", err)
	}

	balanceBefore, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance before: %v", err)
	}

	balanceAfter, err := NewMoney("120.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance after: %v", err)
	}

	_, err = NewWalletLedgerEntry(
		"0192f300-27dd-7d3f-8071-5f8685deef37",
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f298-345e-7e38-af88-e43f851a819d",
		LedgerDirectionCredit,
		money,
		balanceBefore,
		balanceAfter,
		time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
	)
	if !errors.Is(err, ErrInconsistentLedgerBalance) {
		t.Fatalf(
			"NewWalletLedgerEntry() error = %v, want ErrInconsistentLedgerBalance",
			err,
		)
	}
}

func TestNewWalletLedgerEntryCreatesValidDebit(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for money: %v", err)
	}

	balanceBefore, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance before: %v", err)
	}

	balanceAfter, err := NewMoney("75.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance after: %v", err)
	}

	entry, err := NewWalletLedgerEntry(
		"0192f300-27dd-7d3f-8071-5f8685deef37",
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f298-345e-7e38-af88-e43f851a819d",
		LedgerDirectionDebit,
		money,
		balanceBefore,
		balanceAfter,
		time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewWalletLedgerEntry() unexpected error: %v", err)
	}

	if got := entry.Direction(); got != LedgerDirectionDebit {
		t.Errorf("WalletLedgerEntry.Direction() = %q, want %q", got, LedgerDirectionDebit)
	}

	if got, want := entry.BalanceAfter().Amount(), "75.00"; got != want {
		t.Errorf("WalletLedgerEntry.BalanceAfter().Amount() = %q, want %q", got, want)
	}
}

func TestNewWalletLedgerEntryRejectsInconsistentDebitBalance(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for money: %v", err)
	}

	balanceBefore, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance before: %v", err)
	}

	balanceAfter, err := NewMoney("80.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance after: %v", err)
	}

	_, err = NewWalletLedgerEntry(
		"0192f300-27dd-7d3f-8071-5f8685deef37",
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f298-345e-7e38-af88-e43f851a819d",
		LedgerDirectionDebit,
		money,
		balanceBefore,
		balanceAfter,
		time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
	)
	if !errors.Is(err, ErrInconsistentLedgerBalance) {
		t.Fatalf(
			"NewWalletLedgerEntry() error = %v, want ErrInconsistentLedgerBalance",
			err,
		)
	}
}

func TestNewWalletLedgerEntryRejectsInvalidDirection(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for money: %v", err)
	}

	balanceBefore, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance before: %v", err)
	}

	balanceAfter, err := NewMoney("125.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance after: %v", err)
	}

	for _, direction := range []LedgerDirection{"", "TRANSFER"} {
		direction := direction
		t.Run(string(direction), func(t *testing.T) {
			t.Parallel()

			_, err := NewWalletLedgerEntry(
				"0192f300-27dd-7d3f-8071-5f8685deef37",
				"0192f291-27dd-7d3f-8071-5f8685deef37",
				"0192f298-345e-7e38-af88-e43f851a819d",
				direction,
				money,
				balanceBefore,
				balanceAfter,
				time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
			)
			if !errors.Is(err, ErrInvalidLedgerDirection) {
				t.Fatalf(
					"NewWalletLedgerEntry() error = %v, want ErrInvalidLedgerDirection",
					err,
				)
			}
		})
	}
}

func TestNewWalletLedgerEntryRejectsEmptyIdentifiers(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for money: %v", err)
	}

	balanceBefore, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance before: %v", err)
	}

	balanceAfter, err := NewMoney("125.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance after: %v", err)
	}

	validEntryID := "0192f300-27dd-7d3f-8071-5f8685deef37"
	validWalletID := "0192f291-27dd-7d3f-8071-5f8685deef37"
	validTransactionID := "0192f298-345e-7e38-af88-e43f851a819d"

	testCases := []struct {
		name          string
		entryID       string
		walletID      string
		transactionID string
		wantErr       error
	}{
		{
			name:          "empty entry ID",
			entryID:       "",
			walletID:      validWalletID,
			transactionID: validTransactionID,
			wantErr:       ErrInvalidLedgerEntryID,
		},
		{
			name:          "empty wallet ID",
			entryID:       validEntryID,
			walletID:      "",
			transactionID: validTransactionID,
			wantErr:       ErrInvalidWalletID,
		},
		{
			name:          "empty transaction ID",
			entryID:       validEntryID,
			walletID:      validWalletID,
			transactionID: "",
			wantErr:       ErrInvalidTransactionID,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewWalletLedgerEntry(
				testCase.entryID,
				testCase.walletID,
				testCase.transactionID,
				LedgerDirectionCredit,
				money,
				balanceBefore,
				balanceAfter,
				time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
			)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("NewWalletLedgerEntry() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestNewWalletLedgerEntryRejectsNonPositiveMoney(t *testing.T) {
	t.Parallel()

	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for zero: %v", err)
	}

	one, err := NewMoney("1.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for one: %v", err)
	}

	negative, err := zero.Subtract(one)
	if err != nil {
		t.Fatalf("Money.Subtract() unexpected setup error: %v", err)
	}

	balanceBefore, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance before: %v", err)
	}

	negativeBalanceAfter, err := NewMoney("99.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for negative case balance after: %v", err)
	}

	testCases := []struct {
		name         string
		money        Money
		balanceAfter Money
	}{
		{name: "zero", money: zero, balanceAfter: balanceBefore},
		{name: "negative", money: negative, balanceAfter: negativeBalanceAfter},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewWalletLedgerEntry(
				"0192f300-27dd-7d3f-8071-5f8685deef37",
				"0192f291-27dd-7d3f-8071-5f8685deef37",
				"0192f298-345e-7e38-af88-e43f851a819d",
				LedgerDirectionCredit,
				testCase.money,
				balanceBefore,
				testCase.balanceAfter,
				time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
			)
			if !errors.Is(err, ErrNonPositiveAmount) {
				t.Fatalf("NewWalletLedgerEntry() error = %v, want ErrNonPositiveAmount", err)
			}
		})
	}
}

func TestNewWalletLedgerEntryRejectsInvalidMonetaryValues(t *testing.T) {
	t.Parallel()

	brl25, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for BRL money: %v", err)
	}

	brl100, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for BRL balance before: %v", err)
	}

	brl125, err := NewMoney("125.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for BRL balance after: %v", err)
	}

	usd25, err := NewMoney("25.00", "USD")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for USD money: %v", err)
	}

	usd125, err := NewMoney("125.00", "USD")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for USD balance after: %v", err)
	}

	var uninitialized Money
	testCases := []struct {
		name          string
		money         Money
		balanceBefore Money
		balanceAfter  Money
		wantErr       error
	}{
		{
			name:          "uninitialized money",
			money:         uninitialized,
			balanceBefore: brl100,
			balanceAfter:  brl125,
			wantErr:       ErrInvalidCurrency,
		},
		{
			name:          "uninitialized balance before",
			money:         brl25,
			balanceBefore: uninitialized,
			balanceAfter:  brl125,
			wantErr:       ErrInvalidCurrency,
		},
		{
			name:          "uninitialized balance after",
			money:         brl25,
			balanceBefore: brl100,
			balanceAfter:  uninitialized,
			wantErr:       ErrInvalidCurrency,
		},
		{
			name:          "money currency differs",
			money:         usd25,
			balanceBefore: brl100,
			balanceAfter:  brl125,
			wantErr:       ErrCurrencyMismatch,
		},
		{
			name:          "balance after currency differs",
			money:         brl25,
			balanceBefore: brl100,
			balanceAfter:  usd125,
			wantErr:       ErrCurrencyMismatch,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewWalletLedgerEntry(
				"0192f300-27dd-7d3f-8071-5f8685deef37",
				"0192f291-27dd-7d3f-8071-5f8685deef37",
				"0192f298-345e-7e38-af88-e43f851a819d",
				LedgerDirectionCredit,
				testCase.money,
				testCase.balanceBefore,
				testCase.balanceAfter,
				time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
			)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("NewWalletLedgerEntry() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestNewWalletLedgerEntryRejectsZeroTimestamp(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for money: %v", err)
	}

	balanceBefore, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance before: %v", err)
	}

	balanceAfter, err := NewMoney("125.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance after: %v", err)
	}

	_, err = NewWalletLedgerEntry(
		"0192f300-27dd-7d3f-8071-5f8685deef37",
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f298-345e-7e38-af88-e43f851a819d",
		LedgerDirectionCredit,
		money,
		balanceBefore,
		balanceAfter,
		time.Time{},
	)
	if !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("NewWalletLedgerEntry() error = %v, want ErrInvalidTimestamp", err)
	}
}

func TestNewWalletLedgerEntryNormalizesTimestampToUTC(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for money: %v", err)
	}

	balanceBefore, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance before: %v", err)
	}

	balanceAfter, err := NewMoney("125.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for balance after: %v", err)
	}

	localTime := time.Date(
		2026,
		time.September,
		30,
		12,
		0,
		0,
		0,
		time.FixedZone("BRT", -3*60*60),
	)

	entry, err := NewWalletLedgerEntry(
		"0192f300-27dd-7d3f-8071-5f8685deef37",
		"0192f291-27dd-7d3f-8071-5f8685deef37",
		"0192f298-345e-7e38-af88-e43f851a819d",
		LedgerDirectionCredit,
		money,
		balanceBefore,
		balanceAfter,
		localTime,
	)
	if err != nil {
		t.Fatalf("NewWalletLedgerEntry() unexpected error: %v", err)
	}

	if got, want := entry.CreatedAt(), localTime.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("WalletLedgerEntry.CreatedAt() = %v (%v), want %v (UTC)", got, got.Location(), want)
	}
}
