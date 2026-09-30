package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewExternalWagerTransactionCreatesPendingBet(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	now := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	params := NewExternalWagerTransactionParams{
		ID:                    "0192f298-345e-7e38-af88-e43f851a819d",
		ExternalTransactionID: "transaction-123",
		ProviderID:            "provider-a",
		IdempotencyKey:        "provider-a:transaction-123",
		PayloadHash:           "sha256:33ab8c900f8de593617e2f596c11f72c",
		WalletID:              "0192f291-27dd-7d3f-8071-5f8685deef37",
		PlayerID:              "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		RoundID:               "round-987",
		GameID:                "fortune-chimp",
		Kind:                  WagerTransactionKindBet,
		Money:                 money,
		OccurredAt:            now,
	}

	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected error: %v", err)
	}

	if got := transaction.ID(); got != params.ID {
		t.Errorf("WagerTransaction.ID() = %q, want %q", got, params.ID)
	}

	if got := transaction.ExternalTransactionID(); got != params.ExternalTransactionID {
		t.Errorf("WagerTransaction.ExternalTransactionID() = %q, want %q", got, params.ExternalTransactionID)
	}

	if got := transaction.ProviderID(); got != params.ProviderID {
		t.Errorf("WagerTransaction.ProviderID() = %q, want %q", got, params.ProviderID)
	}

	if got := transaction.IdempotencyKey(); got != params.IdempotencyKey {
		t.Errorf("WagerTransaction.IdempotencyKey() = %q, want %q", got, params.IdempotencyKey)
	}

	if got := transaction.PayloadHash(); got != params.PayloadHash {
		t.Errorf("WagerTransaction.PayloadHash() = %q, want %q", got, params.PayloadHash)
	}

	if got := transaction.WalletID(); got != params.WalletID {
		t.Errorf("WagerTransaction.WalletID() = %q, want %q", got, params.WalletID)
	}

	if got := transaction.PlayerID(); got != params.PlayerID {
		t.Errorf("WagerTransaction.PlayerID() = %q, want %q", got, params.PlayerID)
	}

	if got := transaction.RoundID(); got != params.RoundID {
		t.Errorf("WagerTransaction.RoundID() = %q, want %q", got, params.RoundID)
	}

	if got := transaction.GameID(); got != params.GameID {
		t.Errorf("WagerTransaction.GameID() = %q, want %q", got, params.GameID)
	}

	if got := transaction.Kind(); got != WagerTransactionKindBet {
		t.Errorf("WagerTransaction.Kind() = %q, want %q", got, WagerTransactionKindBet)
	}

	if got, want := transaction.Money().Amount(), "25.00"; got != want {
		t.Errorf("WagerTransaction.Money().Amount() = %q, want %q", got, want)
	}

	if got := transaction.Status(); got != WagerTransactionStatusPending {
		t.Errorf("WagerTransaction.Status() = %q, want %q", got, WagerTransactionStatusPending)
	}

	if got := transaction.CreatedAt(); !got.Equal(now) || got.Location() != time.UTC {
		t.Errorf("WagerTransaction.CreatedAt() = %v (%v), want %v (UTC)", got, got.Location(), now)
	}

	if got := transaction.UpdatedAt(); !got.Equal(now) || got.Location() != time.UTC {
		t.Errorf("WagerTransaction.UpdatedAt() = %v (%v), want %v (UTC)", got, got.Location(), now)
	}
}

func TestNewExternalWagerTransactionRejectsMissingMetadata(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	base := NewExternalWagerTransactionParams{
		ID:                    "0192f298-345e-7e38-af88-e43f851a819d",
		ExternalTransactionID: "transaction-123",
		ProviderID:            "provider-a",
		IdempotencyKey:        "provider-a:transaction-123",
		PayloadHash:           "sha256:33ab8c900f8de593617e2f596c11f72c",
		WalletID:              "0192f291-27dd-7d3f-8071-5f8685deef37",
		PlayerID:              "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		RoundID:               "round-987",
		GameID:                "fortune-chimp",
		Kind:                  WagerTransactionKindBet,
		Money:                 money,
		OccurredAt:            time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
	}

	testCases := []struct {
		name    string
		mutate  func(*NewExternalWagerTransactionParams)
		wantErr error
	}{
		{
			name:    "missing transaction ID",
			mutate:  func(params *NewExternalWagerTransactionParams) { params.ID = "" },
			wantErr: ErrInvalidWagerTransactionID,
		},
		{
			name: "missing external transaction ID",
			mutate: func(params *NewExternalWagerTransactionParams) {
				params.ExternalTransactionID = ""
			},
			wantErr: ErrInvalidExternalTransactionID,
		},
		{
			name:    "missing provider ID",
			mutate:  func(params *NewExternalWagerTransactionParams) { params.ProviderID = "" },
			wantErr: ErrInvalidProviderID,
		},
		{
			name: "missing idempotency key",
			mutate: func(params *NewExternalWagerTransactionParams) {
				params.IdempotencyKey = ""
			},
			wantErr: ErrInvalidIdempotencyKey,
		},
		{
			name:    "missing payload hash",
			mutate:  func(params *NewExternalWagerTransactionParams) { params.PayloadHash = "" },
			wantErr: ErrInvalidPayloadHash,
		},
		{
			name:    "missing wallet ID",
			mutate:  func(params *NewExternalWagerTransactionParams) { params.WalletID = "" },
			wantErr: ErrInvalidWalletID,
		},
		{
			name:    "missing player ID",
			mutate:  func(params *NewExternalWagerTransactionParams) { params.PlayerID = "" },
			wantErr: ErrInvalidPlayerID,
		},
		{
			name:    "missing round ID",
			mutate:  func(params *NewExternalWagerTransactionParams) { params.RoundID = "" },
			wantErr: ErrInvalidRoundID,
		},
		{
			name:    "missing game ID",
			mutate:  func(params *NewExternalWagerTransactionParams) { params.GameID = "" },
			wantErr: ErrInvalidGameID,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			params := base
			testCase.mutate(&params)

			_, err := NewExternalWagerTransaction(params)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("NewExternalWagerTransaction() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestNewExternalWagerTransactionRejectsZeroTimestamp(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	params := NewExternalWagerTransactionParams{
		ID:                    "0192f298-345e-7e38-af88-e43f851a819d",
		ExternalTransactionID: "transaction-123",
		ProviderID:            "provider-a",
		IdempotencyKey:        "provider-a:transaction-123",
		PayloadHash:           "sha256:33ab8c900f8de593617e2f596c11f72c",
		WalletID:              "0192f291-27dd-7d3f-8071-5f8685deef37",
		PlayerID:              "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		RoundID:               "round-987",
		GameID:                "fortune-chimp",
		Kind:                  WagerTransactionKindBet,
		Money:                 money,
		OccurredAt:            time.Time{},
	}

	_, err = NewExternalWagerTransaction(params)
	if !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("NewExternalWagerTransaction() error = %v, want ErrInvalidTimestamp", err)
	}
}

func TestNewExternalWagerTransactionRejectsInternalAndUnknownKinds(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	base := NewExternalWagerTransactionParams{
		ID:                    "0192f298-345e-7e38-af88-e43f851a819d",
		ExternalTransactionID: "transaction-123",
		ProviderID:            "provider-a",
		IdempotencyKey:        "provider-a:transaction-123",
		PayloadHash:           "sha256:33ab8c900f8de593617e2f596c11f72c",
		WalletID:              "0192f291-27dd-7d3f-8071-5f8685deef37",
		PlayerID:              "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		RoundID:               "round-987",
		GameID:                "fortune-chimp",
		Money:                 money,
		OccurredAt:            time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
	}

	for _, kind := range []WagerTransactionKind{
		WagerTransactionKindOpening,
		WagerTransactionKind("BONUS"),
	} {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			params := base
			params.Kind = kind

			_, err := NewExternalWagerTransaction(params)
			if !errors.Is(err, ErrInvalidWagerTransactionKind) {
				t.Fatalf(
					"NewExternalWagerTransaction() error = %v, want ErrInvalidWagerTransactionKind",
					err,
				)
			}
		})
	}
}

func TestNewExternalWagerTransactionRejectsInvalidAmountForKind(t *testing.T) {
	t.Parallel()

	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for zero: %v", err)
	}

	positive, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for positive amount: %v", err)
	}

	testCases := []struct {
		name    string
		kind    WagerTransactionKind
		money   Money
		wantErr error
	}{
		{name: "BET with zero", kind: WagerTransactionKindBet, money: zero, wantErr: ErrNonPositiveAmount},
		{name: "WIN with zero", kind: WagerTransactionKindWin, money: zero, wantErr: ErrNonPositiveAmount},
		{name: "REFUND with zero", kind: WagerTransactionKindRefund, money: zero, wantErr: ErrNonPositiveAmount},
		{name: "ROLLBACK with zero", kind: WagerTransactionKindRollback, money: zero, wantErr: ErrNonPositiveAmount},
		{name: "LOSS with positive amount", kind: WagerTransactionKindLoss, money: positive, wantErr: ErrLossAmountMustBeZero},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			params := validExternalWagerTransactionParams(t)
			params.Kind = testCase.kind
			params.Money = testCase.money

			_, err := NewExternalWagerTransaction(params)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("NewExternalWagerTransaction() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestNewExternalWagerTransactionCreatesPendingLossWithZero(t *testing.T) {
	t.Parallel()

	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindLoss
	params.Money = zero

	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected error: %v", err)
	}

	if got := transaction.Kind(); got != WagerTransactionKindLoss {
		t.Errorf("WagerTransaction.Kind() = %q, want %q", got, WagerTransactionKindLoss)
	}

	if got, want := transaction.Money().Amount(), "0.00"; got != want {
		t.Errorf("WagerTransaction.Money().Amount() = %q, want %q", got, want)
	}

	if got := transaction.Status(); got != WagerTransactionStatusPending {
		t.Errorf("WagerTransaction.Status() = %q, want %q", got, WagerTransactionStatusPending)
	}
}

func TestNewExternalWagerTransactionRejectsReversalWithoutReference(t *testing.T) {
	t.Parallel()

	for _, kind := range []WagerTransactionKind{
		WagerTransactionKindRefund,
		WagerTransactionKindRollback,
	} {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			params := validExternalWagerTransactionParams(t)
			params.Kind = kind

			_, err := NewExternalWagerTransaction(params)
			if !errors.Is(err, ErrReferenceExternalTransactionIDRequired) {
				t.Fatalf(
					"NewExternalWagerTransaction() error = %v, want ErrReferenceExternalTransactionIDRequired",
					err,
				)
			}
		})
	}
}

func validExternalWagerTransactionParams(t *testing.T) NewExternalWagerTransactionParams {
	t.Helper()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	return NewExternalWagerTransactionParams{
		ID:                    "0192f298-345e-7e38-af88-e43f851a819d",
		ExternalTransactionID: "transaction-123",
		ProviderID:            "provider-a",
		IdempotencyKey:        "provider-a:transaction-123",
		PayloadHash:           "sha256:33ab8c900f8de593617e2f596c11f72c",
		WalletID:              "0192f291-27dd-7d3f-8071-5f8685deef37",
		PlayerID:              "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
		RoundID:               "round-987",
		GameID:                "fortune-chimp",
		Kind:                  WagerTransactionKindBet,
		Money:                 money,
		OccurredAt:            time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC),
	}
}
