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

func TestNewExternalWagerTransactionCreatesPendingRefundWithReference(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "bet-transaction-456"

	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected error: %v", err)
	}

	if got := transaction.Kind(); got != WagerTransactionKindRefund {
		t.Errorf("WagerTransaction.Kind() = %q, want %q", got, WagerTransactionKindRefund)
	}

	if got := transaction.ReferenceExternalTransactionID(); got != params.ReferenceExternalTransactionID {
		t.Errorf(
			"WagerTransaction.ReferenceExternalTransactionID() = %q, want %q",
			got,
			params.ReferenceExternalTransactionID,
		)
	}

	if got := transaction.Status(); got != WagerTransactionStatusPending {
		t.Errorf("WagerTransaction.Status() = %q, want %q", got, WagerTransactionStatusPending)
	}
}

func TestWagerTransactionMarksPendingAsProcessed(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	resultBalance, err := NewMoney("975.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for result balance: %v", err)
	}

	processedAt := params.OccurredAt.Add(time.Second)
	if err := transaction.MarkProcessed(resultBalance, processedAt); err != nil {
		t.Fatalf("WagerTransaction.MarkProcessed() unexpected error: %v", err)
	}

	if got := transaction.Status(); got != WagerTransactionStatusProcessed {
		t.Errorf("WagerTransaction.Status() = %q, want %q", got, WagerTransactionStatusProcessed)
	}

	gotBalance, ok := transaction.ResultBalance()
	if !ok {
		t.Fatal("WagerTransaction.ResultBalance() reported no persisted result")
	}

	if got, want := gotBalance.Amount(), "975.00"; got != want {
		t.Errorf("WagerTransaction.ResultBalance().Amount() = %q, want %q", got, want)
	}

	if got := transaction.CreatedAt(); !got.Equal(params.OccurredAt) {
		t.Errorf("WagerTransaction.CreatedAt() changed to %v, want %v", got, params.OccurredAt)
	}

	if got := transaction.UpdatedAt(); !got.Equal(processedAt) || got.Location() != time.UTC {
		t.Errorf("WagerTransaction.UpdatedAt() = %v (%v), want %v (UTC)", got, got.Location(), processedAt)
	}
}

func TestWagerTransactionCannotProcessTerminalTransactionAgain(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	firstBalance, err := NewMoney("975.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for first balance: %v", err)
	}

	firstProcessedAt := params.OccurredAt.Add(time.Second)
	if err := transaction.MarkProcessed(firstBalance, firstProcessedAt); err != nil {
		t.Fatalf("WagerTransaction.MarkProcessed() unexpected setup error: %v", err)
	}

	secondBalance, err := NewMoney("950.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for second balance: %v", err)
	}

	err = transaction.MarkProcessed(secondBalance, firstProcessedAt.Add(time.Second))
	if !errors.Is(err, ErrInvalidWagerTransactionTransition) {
		t.Fatalf(
			"WagerTransaction.MarkProcessed() error = %v, want ErrInvalidWagerTransactionTransition",
			err,
		)
	}

	if got := transaction.Status(); got != WagerTransactionStatusProcessed {
		t.Errorf("WagerTransaction.Status() = %q, want %q", got, WagerTransactionStatusProcessed)
	}

	gotBalance, ok := transaction.ResultBalance()
	if !ok {
		t.Fatal("WagerTransaction.ResultBalance() reported no persisted result")
	}

	if got, want := gotBalance.Amount(), "975.00"; got != want {
		t.Errorf("WagerTransaction.ResultBalance().Amount() = %q, want original %q", got, want)
	}

	if got := transaction.UpdatedAt(); !got.Equal(firstProcessedAt) {
		t.Errorf("WagerTransaction.UpdatedAt() changed to %v, want %v", got, firstProcessedAt)
	}
}

func TestWagerTransactionRejectsInvalidProcessedResultWithoutChangingState(t *testing.T) {
	t.Parallel()

	brlBalance, err := NewMoney("975.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for BRL balance: %v", err)
	}

	usdBalance, err := NewMoney("975.00", "USD")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for USD balance: %v", err)
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
	baseTime := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	testCases := []struct {
		name          string
		resultBalance Money
		processedAt   time.Time
		wantErr       error
	}{
		{
			name:          "uninitialized result balance",
			resultBalance: uninitializedBalance,
			processedAt:   baseTime.Add(time.Second),
			wantErr:       ErrInvalidCurrency,
		},
		{
			name:          "different result currency",
			resultBalance: usdBalance,
			processedAt:   baseTime.Add(time.Second),
			wantErr:       ErrCurrencyMismatch,
		},
		{
			name:          "negative result balance",
			resultBalance: negativeBalance,
			processedAt:   baseTime.Add(time.Second),
			wantErr:       ErrNegativeBalance,
		},
		{
			name:          "zero processing timestamp",
			resultBalance: brlBalance,
			processedAt:   time.Time{},
			wantErr:       ErrInvalidTimestamp,
		},
		{
			name:          "stale processing timestamp",
			resultBalance: brlBalance,
			processedAt:   baseTime.Add(-time.Second),
			wantErr:       ErrStaleTimestamp,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			params := validExternalWagerTransactionParams(t)
			params.OccurredAt = baseTime
			transaction, err := NewExternalWagerTransaction(params)
			if err != nil {
				t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
			}

			err = transaction.MarkProcessed(testCase.resultBalance, testCase.processedAt)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("WagerTransaction.MarkProcessed() error = %v, want %v", err, testCase.wantErr)
			}

			if got := transaction.Status(); got != WagerTransactionStatusPending {
				t.Errorf("WagerTransaction.Status() changed to %q, want %q", got, WagerTransactionStatusPending)
			}

			if _, ok := transaction.ResultBalance(); ok {
				t.Error("WagerTransaction.ResultBalance() unexpectedly reported a persisted result")
			}

			if got := transaction.UpdatedAt(); !got.Equal(baseTime) {
				t.Errorf("WagerTransaction.UpdatedAt() changed to %v, want %v", got, baseTime)
			}
		})
	}
}

func TestWagerTransactionMarksReversalAsPendingReference(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "bet-transaction-456"

	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	transitionedAt := params.OccurredAt.Add(time.Second)
	nextAttemptAt := transitionedAt.Add(time.Minute)
	if err := transaction.MarkPendingReference(nextAttemptAt, transitionedAt); err != nil {
		t.Fatalf("WagerTransaction.MarkPendingReference() unexpected error: %v", err)
	}

	if got := transaction.Status(); got != WagerTransactionStatusPendingReference {
		t.Errorf(
			"WagerTransaction.Status() = %q, want %q",
			got,
			WagerTransactionStatusPendingReference,
		)
	}

	if got, want := transaction.ReferenceAttempts(), 1; got != want {
		t.Errorf("WagerTransaction.ReferenceAttempts() = %d, want %d", got, want)
	}

	gotNextAttempt, ok := transaction.NextReferenceAttemptAt()
	if !ok {
		t.Fatal("WagerTransaction.NextReferenceAttemptAt() reported no scheduled retry")
	}

	if !gotNextAttempt.Equal(nextAttemptAt) || gotNextAttempt.Location() != time.UTC {
		t.Errorf(
			"WagerTransaction.NextReferenceAttemptAt() = %v (%v), want %v (UTC)",
			gotNextAttempt,
			gotNextAttempt.Location(),
			nextAttemptAt,
		)
	}

	if got := transaction.UpdatedAt(); !got.Equal(transitionedAt) || got.Location() != time.UTC {
		t.Errorf("WagerTransaction.UpdatedAt() = %v (%v), want %v (UTC)", got, got.Location(), transitionedAt)
	}
}

func TestWagerTransactionRejectsPendingReferenceForNonReversal(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	transitionedAt := params.OccurredAt.Add(time.Second)
	nextAttemptAt := transitionedAt.Add(time.Minute)
	err = transaction.MarkPendingReference(nextAttemptAt, transitionedAt)
	if !errors.Is(err, ErrInvalidWagerTransactionTransition) {
		t.Fatalf(
			"WagerTransaction.MarkPendingReference() error = %v, want ErrInvalidWagerTransactionTransition",
			err,
		)
	}

	if got := transaction.Status(); got != WagerTransactionStatusPending {
		t.Errorf("WagerTransaction.Status() changed to %q, want %q", got, WagerTransactionStatusPending)
	}

	if got := transaction.ReferenceAttempts(); got != 0 {
		t.Errorf("WagerTransaction.ReferenceAttempts() changed to %d, want 0", got)
	}

	if _, ok := transaction.NextReferenceAttemptAt(); ok {
		t.Error("WagerTransaction.NextReferenceAttemptAt() unexpectedly reported a scheduled retry")
	}

	if got := transaction.UpdatedAt(); !got.Equal(params.OccurredAt) {
		t.Errorf("WagerTransaction.UpdatedAt() changed to %v, want %v", got, params.OccurredAt)
	}
}

func TestWagerTransactionRejectsInvalidPendingReferenceScheduleWithoutChangingState(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	validNow := baseTime.Add(time.Second)
	testCases := []struct {
		name          string
		nextAttemptAt time.Time
		now           time.Time
		wantErr       error
	}{
		{
			name:          "zero transition timestamp",
			nextAttemptAt: validNow.Add(time.Minute),
			now:           time.Time{},
			wantErr:       ErrInvalidTimestamp,
		},
		{
			name:          "stale transition timestamp",
			nextAttemptAt: validNow.Add(time.Minute),
			now:           baseTime.Add(-time.Second),
			wantErr:       ErrStaleTimestamp,
		},
		{
			name:          "zero next attempt timestamp",
			nextAttemptAt: time.Time{},
			now:           validNow,
			wantErr:       ErrInvalidTimestamp,
		},
		{
			name:          "next attempt equal to transition",
			nextAttemptAt: validNow,
			now:           validNow,
			wantErr:       ErrInvalidReferenceRetrySchedule,
		},
		{
			name:          "next attempt before transition",
			nextAttemptAt: validNow.Add(-time.Nanosecond),
			now:           validNow,
			wantErr:       ErrInvalidReferenceRetrySchedule,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			params := validExternalWagerTransactionParams(t)
			params.Kind = WagerTransactionKindRefund
			params.ReferenceExternalTransactionID = "bet-transaction-456"
			params.OccurredAt = baseTime

			transaction, err := NewExternalWagerTransaction(params)
			if err != nil {
				t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
			}

			err = transaction.MarkPendingReference(testCase.nextAttemptAt, testCase.now)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("WagerTransaction.MarkPendingReference() error = %v, want %v", err, testCase.wantErr)
			}

			if got := transaction.Status(); got != WagerTransactionStatusPending {
				t.Errorf("WagerTransaction.Status() changed to %q, want %q", got, WagerTransactionStatusPending)
			}

			if got := transaction.ReferenceAttempts(); got != 0 {
				t.Errorf("WagerTransaction.ReferenceAttempts() changed to %d, want 0", got)
			}

			if _, ok := transaction.NextReferenceAttemptAt(); ok {
				t.Error("WagerTransaction.NextReferenceAttemptAt() unexpectedly reported a scheduled retry")
			}

			if got := transaction.UpdatedAt(); !got.Equal(baseTime) {
				t.Errorf("WagerTransaction.UpdatedAt() changed to %v, want %v", got, baseTime)
			}
		})
	}
}

func TestWagerTransactionMarksPendingReferenceAsProcessed(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "bet-transaction-456"

	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	pendingAt := params.OccurredAt.Add(time.Second)
	nextAttemptAt := pendingAt.Add(time.Minute)

	err = transaction.MarkPendingReference(nextAttemptAt, pendingAt)
	if err != nil {
		t.Fatalf("MarkPendingReference() unexpected setup error: %v", err)
	}

	resultBalance, err := NewMoney("1025.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	processedAt := nextAttemptAt
	referenceTransactionID := "internal-bet-transaction-456"
	if err := transaction.ResolveReference(referenceTransactionID, processedAt); err != nil {
		t.Fatalf("ResolveReference() unexpected error: %v", err)
	}

	err = transaction.MarkProcessed(resultBalance, processedAt)
	if err != nil {
		t.Fatalf("MarkProcessed() unexpected error: %v", err)
	}

	if got := transaction.Status(); got != WagerTransactionStatusProcessed {
		t.Errorf(
			"WagerTransaction.Status() = %q, want %q",
			got,
			WagerTransactionStatusProcessed,
		)
	}

	if got := transaction.ReferenceTransactionID(); got != referenceTransactionID {
		t.Errorf("ReferenceTransactionID() = %q, want %q", got, referenceTransactionID)
	}

	gotBalance, ok := transaction.ResultBalance()
	if !ok {
		t.Fatal("ResultBalance() reported no persisted result")
	}

	if got, want := gotBalance.Amount(), "1025.00"; got != want {
		t.Errorf("ResultBalance().Amount() = %q, want %q", got, want)
	}

	if _, ok := transaction.NextReferenceAttemptAt(); ok {
		t.Error("NextReferenceAttemptAt() still reports a retry after processing")
	}

	if got := transaction.UpdatedAt(); !got.Equal(processedAt) {
		t.Errorf("UpdatedAt() = %v, want %v", got, processedAt)
	}
}

func TestWagerTransactionReschedulesPendingReference(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "bet-transaction-456"

	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	firstTransitionAt := params.OccurredAt.Add(time.Second)
	firstAttemptAt := firstTransitionAt.Add(time.Minute)

	err = transaction.MarkPendingReference(firstAttemptAt, firstTransitionAt)
	if err != nil {
		t.Fatalf("MarkPendingReference() unexpected setup error: %v", err)
	}

	rescheduledAt := firstAttemptAt
	secondAttemptAt := rescheduledAt.Add(2 * time.Minute)

	err = transaction.RescheduleReference(secondAttemptAt, rescheduledAt)
	if err != nil {
		t.Fatalf("RescheduleReference() unexpected error: %v", err)
	}

	if got := transaction.Status(); got != WagerTransactionStatusPendingReference {
		t.Errorf(
			"Status() = %q, want %q",
			got,
			WagerTransactionStatusPendingReference,
		)
	}

	if got, want := transaction.ReferenceAttempts(), 2; got != want {
		t.Errorf("ReferenceAttempts() = %d, want %d", got, want)
	}

	gotNextAttempt, ok := transaction.NextReferenceAttemptAt()
	if !ok {
		t.Fatal("NextReferenceAttemptAt() reported no scheduled retry")
	}

	if !gotNextAttempt.Equal(secondAttemptAt) {
		t.Errorf(
			"NextReferenceAttemptAt() = %v, want %v",
			gotNextAttempt,
			secondAttemptAt,
		)
	}

	if got := transaction.UpdatedAt(); !got.Equal(rescheduledAt) {
		t.Errorf("UpdatedAt() = %v, want %v", got, rescheduledAt)
	}
}

func TestWagerTransactionRejectsReferenceRescheduleBeforeScheduledAttempt(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "bet-transaction-456"

	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	pendingAt := params.OccurredAt.Add(time.Second)
	firstAttemptAt := pendingAt.Add(time.Minute)

	err = transaction.MarkPendingReference(firstAttemptAt, pendingAt)
	if err != nil {
		t.Fatalf("MarkPendingReference() unexpected setup error: %v", err)
	}

	earlyRetryAt := firstAttemptAt.Add(-time.Second)
	secondAttemptAt := firstAttemptAt.Add(2 * time.Minute)

	err = transaction.RescheduleReference(secondAttemptAt, earlyRetryAt)
	if !errors.Is(err, ErrReferenceRetryNotDue) {
		t.Fatalf(
			"RescheduleReference() error = %v, want ErrReferenceRetryNotDue",
			err,
		)
	}

	if got := transaction.Status(); got != WagerTransactionStatusPendingReference {
		t.Errorf(
			"Status() changed to %q, want %q",
			got,
			WagerTransactionStatusPendingReference,
		)
	}

	if got, want := transaction.ReferenceAttempts(), 1; got != want {
		t.Errorf("ReferenceAttempts() changed to %d, want %d", got, want)
	}

	gotNextAttempt, ok := transaction.NextReferenceAttemptAt()
	if !ok {
		t.Fatal("NextReferenceAttemptAt() reported no scheduled retry")
	}

	if !gotNextAttempt.Equal(firstAttemptAt) {
		t.Errorf(
			"NextReferenceAttemptAt() changed to %v, want %v",
			gotNextAttempt,
			firstAttemptAt,
		)
	}

	if got := transaction.UpdatedAt(); !got.Equal(pendingAt) {
		t.Errorf("UpdatedAt() changed to %v, want %v", got, pendingAt)
	}
}

func TestWagerTransactionRejectsPendingReferenceWhenReferenceIsNotFound(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "missing-bet-transaction"

	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	pendingAt := params.OccurredAt.Add(time.Second)
	nextAttemptAt := pendingAt.Add(time.Minute)

	err = transaction.MarkPendingReference(nextAttemptAt, pendingAt)
	if err != nil {
		t.Fatalf("MarkPendingReference() unexpected setup error: %v", err)
	}

	rejectedAt := nextAttemptAt
	err = transaction.MarkRejected(
		WagerTransactionFailureCodeReferenceNotFound,
		rejectedAt,
	)
	if err != nil {
		t.Fatalf("MarkRejected() unexpected error: %v", err)
	}

	if got := transaction.Status(); got != WagerTransactionStatusRejected {
		t.Errorf(
			"Status() = %q, want %q",
			got,
			WagerTransactionStatusRejected,
		)
	}

	if got := transaction.FailureCode(); got != WagerTransactionFailureCodeReferenceNotFound {
		t.Errorf(
			"FailureCode() = %q, want %q",
			got,
			WagerTransactionFailureCodeReferenceNotFound,
		)
	}

	if _, ok := transaction.NextReferenceAttemptAt(); ok {
		t.Error("NextReferenceAttemptAt() still reports a retry after rejection")
	}

	if _, ok := transaction.ResultBalance(); ok {
		t.Error("ResultBalance() unexpectedly reported a result after rejection")
	}

	if got := transaction.UpdatedAt(); !got.Equal(rejectedAt) {
		t.Errorf("UpdatedAt() = %v, want %v", got, rejectedAt)
	}
}

func TestWagerTransactionMarksPendingAsFailed(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	failedAt := params.OccurredAt.Add(time.Second)
	err = transaction.MarkFailed(
		WagerTransactionFailureCodeInfrastructureFailure,
		failedAt,
	)
	if err != nil {
		t.Fatalf("MarkFailed() unexpected error: %v", err)
	}

	if got := transaction.Status(); got != WagerTransactionStatusFailed {
		t.Errorf(
			"Status() = %q, want %q",
			got,
			WagerTransactionStatusFailed,
		)
	}

	if got := transaction.FailureCode(); got != WagerTransactionFailureCodeInfrastructureFailure {
		t.Errorf(
			"FailureCode() = %q, want %q",
			got,
			WagerTransactionFailureCodeInfrastructureFailure,
		)
	}

	if _, ok := transaction.ResultBalance(); ok {
		t.Error("ResultBalance() unexpectedly reported a result after permanent failure")
	}

	if _, ok := transaction.NextReferenceAttemptAt(); ok {
		t.Error("NextReferenceAttemptAt() unexpectedly reported a retry after permanent failure")
	}

	if got := transaction.UpdatedAt(); !got.Equal(failedAt) {
		t.Errorf("UpdatedAt() = %v, want %v", got, failedAt)
	}
}

func TestWagerTransactionTerminalStatesRejectFurtherTransitions(t *testing.T) {
	t.Parallel()

	resultBalance, err := NewMoney("975.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	testCases := []struct {
		name           string
		terminalStatus WagerTransactionStatus
		makeTerminal   func(*WagerTransaction, time.Time) error
		tryTransition  func(*WagerTransaction, time.Time) error
	}{
		{
			name:           "processed cannot become rejected",
			terminalStatus: WagerTransactionStatusProcessed,
			makeTerminal: func(transaction *WagerTransaction, now time.Time) error {
				return transaction.MarkProcessed(resultBalance, now)
			},
			tryTransition: func(transaction *WagerTransaction, now time.Time) error {
				return transaction.MarkRejected(
					WagerTransactionFailureCodeReferenceNotFound,
					now,
				)
			},
		},
		{
			name:           "rejected cannot become failed",
			terminalStatus: WagerTransactionStatusRejected,
			makeTerminal: func(transaction *WagerTransaction, now time.Time) error {
				return transaction.MarkRejected(
					WagerTransactionFailureCodeReferenceNotFound,
					now,
				)
			},
			tryTransition: func(transaction *WagerTransaction, now time.Time) error {
				return transaction.MarkFailed(
					WagerTransactionFailureCodeInfrastructureFailure,
					now,
				)
			},
		},
		{
			name:           "failed cannot become processed",
			terminalStatus: WagerTransactionStatusFailed,
			makeTerminal: func(transaction *WagerTransaction, now time.Time) error {
				return transaction.MarkFailed(
					WagerTransactionFailureCodeInfrastructureFailure,
					now,
				)
			},
			tryTransition: func(transaction *WagerTransaction, now time.Time) error {
				return transaction.MarkProcessed(resultBalance, now)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			params := validExternalWagerTransactionParams(t)
			transaction, err := NewExternalWagerTransaction(params)
			if err != nil {
				t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
			}

			terminalAt := params.OccurredAt.Add(time.Second)
			if err := testCase.makeTerminal(&transaction, terminalAt); err != nil {
				t.Fatalf("terminal transition unexpected setup error: %v", err)
			}

			err = testCase.tryTransition(&transaction, terminalAt.Add(time.Second))
			if !errors.Is(err, ErrInvalidWagerTransactionTransition) {
				t.Fatalf(
					"transition from terminal state error = %v, want ErrInvalidWagerTransactionTransition",
					err,
				)
			}

			if got := transaction.Status(); got != testCase.terminalStatus {
				t.Errorf("Status() changed to %q, want %q", got, testCase.terminalStatus)
			}

			if got := transaction.UpdatedAt(); !got.Equal(terminalAt) {
				t.Errorf("UpdatedAt() changed to %v, want %v", got, terminalAt)
			}
		})
	}
}

func TestRehydrateWagerTransactionRestoresProcessedTransaction(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for transaction money: %v", err)
	}

	resultBalance, err := NewMoney("975.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for result balance: %v", err)
	}

	createdAt := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Second)
	params := RehydrateWagerTransactionParams{
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
		Status:                WagerTransactionStatusProcessed,
		ResultBalance:         resultBalance,
		HasResultBalance:      true,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}

	transaction, err := RehydrateWagerTransaction(params)
	if err != nil {
		t.Fatalf("RehydrateWagerTransaction() unexpected error: %v", err)
	}

	if got := transaction.ID(); got != params.ID {
		t.Errorf("ID() = %q, want %q", got, params.ID)
	}

	if got := transaction.Status(); got != WagerTransactionStatusProcessed {
		t.Errorf("Status() = %q, want %q", got, WagerTransactionStatusProcessed)
	}

	gotBalance, ok := transaction.ResultBalance()
	if !ok {
		t.Fatal("ResultBalance() reported no persisted result")
	}

	if got, want := gotBalance.Amount(), "975.00"; got != want {
		t.Errorf("ResultBalance().Amount() = %q, want %q", got, want)
	}

	if got := transaction.CreatedAt(); !got.Equal(createdAt) {
		t.Errorf("CreatedAt() = %v, want %v", got, createdAt)
	}

	if got := transaction.UpdatedAt(); !got.Equal(updatedAt) {
		t.Errorf("UpdatedAt() = %v, want %v", got, updatedAt)
	}
}

func TestRehydrateWagerTransactionRejectsProcessedWithoutResultBalance(t *testing.T) {
	t.Parallel()

	params := validProcessedWagerTransactionRehydrateParams(t)
	params.ResultBalance = Money{}
	params.HasResultBalance = false

	_, err := RehydrateWagerTransaction(params)
	if !errors.Is(err, ErrWagerTransactionResultBalanceRequired) {
		t.Fatalf(
			"RehydrateWagerTransaction() error = %v, want ErrWagerTransactionResultBalanceRequired",
			err,
		)
	}
}

func TestRehydrateWagerTransactionRejectsTerminalFailureWithoutFailureCode(t *testing.T) {
	t.Parallel()

	for _, status := range []WagerTransactionStatus{
		WagerTransactionStatusRejected,
		WagerTransactionStatusFailed,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			params := validProcessedWagerTransactionRehydrateParams(t)
			params.Status = status
			params.ResultBalance = Money{}
			params.HasResultBalance = false
			params.FailureCode = ""

			_, err := RehydrateWagerTransaction(params)
			if !errors.Is(err, ErrInvalidWagerTransactionFailureCode) {
				t.Fatalf(
					"RehydrateWagerTransaction() error = %v, want ErrInvalidWagerTransactionFailureCode",
					err,
				)
			}
		})
	}
}

func TestRehydrateWagerTransactionRejectsInvalidPendingReferenceState(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		mutate func(*RehydrateWagerTransactionParams)
	}{
		{
			name: "zero reference attempts",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.ReferenceAttempts = 0
			},
		},
		{
			name: "missing next reference attempt",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.NextReferenceAttemptAt = time.Time{}
				params.HasNextReferenceAttempt = false
			},
		},
		{
			name: "zero next reference attempt timestamp",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.NextReferenceAttemptAt = time.Time{}
				params.HasNextReferenceAttempt = true
			},
		},
		{
			name: "next reference attempt equal to updated at",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.NextReferenceAttemptAt = params.UpdatedAt
			},
		},
		{
			name: "next reference attempt before updated at",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.NextReferenceAttemptAt = params.UpdatedAt.Add(-time.Nanosecond)
			},
		},
		{
			name: "pending reference for non reversal kind",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.Kind = WagerTransactionKindBet
				params.ReferenceExternalTransactionID = ""
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			params := validProcessedWagerTransactionRehydrateParams(t)
			params.Kind = WagerTransactionKindRefund
			params.ReferenceExternalTransactionID = "missing-bet-transaction"
			params.Status = WagerTransactionStatusPendingReference
			params.ResultBalance = Money{}
			params.HasResultBalance = false
			params.ReferenceAttempts = 1
			params.NextReferenceAttemptAt = params.UpdatedAt.Add(time.Minute)
			params.HasNextReferenceAttempt = true
			testCase.mutate(&params)

			_, err := RehydrateWagerTransaction(params)
			if !errors.Is(err, ErrInvalidReferenceRetryState) {
				t.Fatalf(
					"RehydrateWagerTransaction() error = %v, want ErrInvalidReferenceRetryState",
					err,
				)
			}
		})
	}
}

func TestRehydrateWagerTransactionRejectsReferenceRetryOutsidePendingReference(t *testing.T) {
	t.Parallel()

	for _, status := range []WagerTransactionStatus{
		WagerTransactionStatusPending,
		WagerTransactionStatusProcessed,
		WagerTransactionStatusRejected,
		WagerTransactionStatusFailed,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			params := validProcessedWagerTransactionRehydrateParams(t)
			params.Status = status
			params.ReferenceAttempts = 1
			params.NextReferenceAttemptAt = params.UpdatedAt.Add(time.Minute)
			params.HasNextReferenceAttempt = true

			switch status {
			case WagerTransactionStatusPending:
				params.ResultBalance = Money{}
				params.HasResultBalance = false
			case WagerTransactionStatusRejected:
				params.ResultBalance = Money{}
				params.HasResultBalance = false
				params.FailureCode = WagerTransactionFailureCodeReferenceNotFound
			case WagerTransactionStatusFailed:
				params.ResultBalance = Money{}
				params.HasResultBalance = false
				params.FailureCode = WagerTransactionFailureCodeInfrastructureFailure
			}

			_, err := RehydrateWagerTransaction(params)
			if !errors.Is(err, ErrInvalidReferenceRetryState) {
				t.Fatalf(
					"RehydrateWagerTransaction() error = %v, want ErrInvalidReferenceRetryState",
					err,
				)
			}
		})
	}
}

func TestRehydrateWagerTransactionRejectsFieldsInIncompatibleStates(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		mutate  func(*RehydrateWagerTransactionParams)
		wantErr error
	}{
		{
			name: "pending with result balance",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.Status = WagerTransactionStatusPending
			},
			wantErr: ErrUnexpectedWagerTransactionResultBalance,
		},
		{
			name: "pending with failure code",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.Status = WagerTransactionStatusPending
				params.ResultBalance = Money{}
				params.HasResultBalance = false
				params.FailureCode = WagerTransactionFailureCodeInfrastructureFailure
			},
			wantErr: ErrUnexpectedWagerTransactionFailureCode,
		},
		{
			name: "processed with failure code",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.FailureCode = WagerTransactionFailureCodeInfrastructureFailure
			},
			wantErr: ErrUnexpectedWagerTransactionFailureCode,
		},
		{
			name: "rejected with result balance",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.Status = WagerTransactionStatusRejected
				params.FailureCode = WagerTransactionFailureCodeReferenceNotFound
			},
			wantErr: ErrUnexpectedWagerTransactionResultBalance,
		},
		{
			name: "failed with result balance",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.Status = WagerTransactionStatusFailed
				params.FailureCode = WagerTransactionFailureCodeInfrastructureFailure
			},
			wantErr: ErrUnexpectedWagerTransactionResultBalance,
		},
		{
			name: "negative reference attempts",
			mutate: func(params *RehydrateWagerTransactionParams) {
				params.ReferenceAttempts = -1
			},
			wantErr: ErrInvalidReferenceRetryState,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			params := validProcessedWagerTransactionRehydrateParams(t)
			testCase.mutate(&params)

			_, err := RehydrateWagerTransaction(params)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf(
					"RehydrateWagerTransaction() error = %v, want %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

func TestRehydrateWagerTransactionRestoresPendingReference(t *testing.T) {
	t.Parallel()

	params := validProcessedWagerTransactionRehydrateParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "bet-transaction-456"
	params.Status = WagerTransactionStatusPendingReference
	params.ResultBalance = Money{}
	params.HasResultBalance = false
	params.ReferenceAttempts = 3
	params.NextReferenceAttemptAt = params.UpdatedAt.Add(time.Minute)
	params.HasNextReferenceAttempt = true

	transaction, err := RehydrateWagerTransaction(params)
	if err != nil {
		t.Fatalf("RehydrateWagerTransaction() unexpected error: %v", err)
	}

	if got := transaction.Status(); got != WagerTransactionStatusPendingReference {
		t.Errorf("Status() = %q, want %q", got, WagerTransactionStatusPendingReference)
	}
	if got := transaction.ReferenceAttempts(); got != 3 {
		t.Errorf("ReferenceAttempts() = %d, want 3", got)
	}
	gotNextAttempt, ok := transaction.NextReferenceAttemptAt()
	if !ok || !gotNextAttempt.Equal(params.NextReferenceAttemptAt) {
		t.Errorf("NextReferenceAttemptAt() = (%v, %v), want (%v, true)", gotNextAttempt, ok, params.NextReferenceAttemptAt)
	}
}

func TestRehydrateWagerTransactionRestoresTerminalRetryHistory(t *testing.T) {
	t.Parallel()

	for _, status := range []WagerTransactionStatus{
		WagerTransactionStatusProcessed,
		WagerTransactionStatusRejected,
		WagerTransactionStatusFailed,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			params := validProcessedWagerTransactionRehydrateParams(t)
			params.Status = status
			params.ReferenceAttempts = 3

			switch status {
			case WagerTransactionStatusRejected:
				params.ResultBalance = Money{}
				params.HasResultBalance = false
				params.FailureCode = WagerTransactionFailureCodeReferenceNotFound
			case WagerTransactionStatusFailed:
				params.ResultBalance = Money{}
				params.HasResultBalance = false
				params.FailureCode = WagerTransactionFailureCodeInfrastructureFailure
			}

			transaction, err := RehydrateWagerTransaction(params)
			if err != nil {
				t.Fatalf("RehydrateWagerTransaction() unexpected error: %v", err)
			}
			if got := transaction.ReferenceAttempts(); got != 3 {
				t.Errorf("ReferenceAttempts() = %d, want 3", got)
			}
			if _, ok := transaction.NextReferenceAttemptAt(); ok {
				t.Error("NextReferenceAttemptAt() unexpectedly reported a scheduled retry")
			}
		})
	}
}

func TestNewOpeningWagerTransactionCreatesProcessedOpening(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("1000.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}
	now := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.FixedZone("BRT", -3*60*60))

	transaction, err := NewOpeningWagerTransaction(NewOpeningWagerTransactionParams{
		ID:         "opening-transaction-1",
		WalletID:   "wallet-1",
		PlayerID:   "player-1",
		Money:      money,
		OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("NewOpeningWagerTransaction() unexpected error: %v", err)
	}

	if got := transaction.Kind(); got != WagerTransactionKindOpening {
		t.Errorf("Kind() = %q, want %q", got, WagerTransactionKindOpening)
	}
	if got := transaction.Status(); got != WagerTransactionStatusProcessed {
		t.Errorf("Status() = %q, want %q", got, WagerTransactionStatusProcessed)
	}
	if transaction.ExternalTransactionID() != "" || transaction.ProviderID() != "" ||
		transaction.IdempotencyKey() != "" || transaction.PayloadHash() != "" ||
		transaction.RoundID() != "" || transaction.GameID() != "" {
		t.Error("opening unexpectedly contains external metadata")
	}
	resultBalance, ok := transaction.ResultBalance()
	if !ok || resultBalance.Amount() != "1000.00" {
		t.Errorf("ResultBalance() = (%q, %v), want (1000.00, true)", resultBalance.Amount(), ok)
	}
	if got := transaction.CreatedAt(); got.Location() != time.UTC || !got.Equal(now) {
		t.Errorf("CreatedAt() = %v (%v), want %v (UTC)", got, got.Location(), now)
	}
}

func TestNewOpeningWagerTransactionRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	positive, err := NewMoney("1000.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for positive money: %v", err)
	}
	zero, err := NewMoney("0.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for zero money: %v", err)
	}
	now := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	base := NewOpeningWagerTransactionParams{
		ID: "opening-transaction-1", WalletID: "wallet-1", PlayerID: "player-1",
		Money: positive, OccurredAt: now,
	}

	testCases := []struct {
		name    string
		mutate  func(*NewOpeningWagerTransactionParams)
		wantErr error
	}{
		{name: "missing ID", mutate: func(p *NewOpeningWagerTransactionParams) { p.ID = "" }, wantErr: ErrInvalidWagerTransactionID},
		{name: "missing wallet ID", mutate: func(p *NewOpeningWagerTransactionParams) { p.WalletID = "" }, wantErr: ErrInvalidWalletID},
		{name: "missing player ID", mutate: func(p *NewOpeningWagerTransactionParams) { p.PlayerID = "" }, wantErr: ErrInvalidPlayerID},
		{name: "uninitialized money", mutate: func(p *NewOpeningWagerTransactionParams) { p.Money = Money{} }, wantErr: ErrInvalidCurrency},
		{name: "zero money", mutate: func(p *NewOpeningWagerTransactionParams) { p.Money = zero }, wantErr: ErrNonPositiveAmount},
		{name: "zero timestamp", mutate: func(p *NewOpeningWagerTransactionParams) { p.OccurredAt = time.Time{} }, wantErr: ErrInvalidTimestamp},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			params := base
			testCase.mutate(&params)
			_, err := NewOpeningWagerTransaction(params)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("NewOpeningWagerTransaction() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestRehydrateWagerTransactionRestoresOpening(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("1000.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}
	now := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	params := RehydrateWagerTransactionParams{
		ID: "opening-transaction-1", WalletID: "wallet-1", PlayerID: "player-1",
		Kind: WagerTransactionKindOpening, Money: money,
		Status: WagerTransactionStatusProcessed, ResultBalance: money, HasResultBalance: true,
		CreatedAt: now, UpdatedAt: now,
	}

	transaction, err := RehydrateWagerTransaction(params)
	if err != nil {
		t.Fatalf("RehydrateWagerTransaction() unexpected error: %v", err)
	}
	if transaction.Kind() != WagerTransactionKindOpening || transaction.Status() != WagerTransactionStatusProcessed {
		t.Errorf("rehydrated opening = (%q, %q), want (OPENING, PROCESSED)", transaction.Kind(), transaction.Status())
	}
}

func TestRehydrateWagerTransactionRejectsInvalidOpeningState(t *testing.T) {
	t.Parallel()

	money, err := NewMoney("1000.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}
	otherBalance, err := NewMoney("999.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}
	now := time.Date(2026, time.September, 30, 15, 0, 0, 0, time.UTC)
	base := RehydrateWagerTransactionParams{
		ID: "opening-transaction-1", WalletID: "wallet-1", PlayerID: "player-1",
		Kind: WagerTransactionKindOpening, Money: money,
		Status: WagerTransactionStatusProcessed, ResultBalance: money, HasResultBalance: true,
		CreatedAt: now, UpdatedAt: now,
	}

	testCases := []struct {
		name    string
		mutate  func(*RehydrateWagerTransactionParams)
		wantErr error
	}{
		{name: "external metadata", mutate: func(p *RehydrateWagerTransactionParams) { p.ProviderID = "provider-a" }, wantErr: ErrUnexpectedExternalWagerTransactionMetadata},
		{name: "non processed status", mutate: func(p *RehydrateWagerTransactionParams) {
			p.Status = WagerTransactionStatusPending
			p.ResultBalance = Money{}
			p.HasResultBalance = false
		}, wantErr: ErrInvalidWagerTransactionTransition},
		{name: "different result balance", mutate: func(p *RehydrateWagerTransactionParams) { p.ResultBalance = otherBalance }, wantErr: ErrOpeningResultBalanceMismatch},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			params := base
			testCase.mutate(&params)
			_, err := RehydrateWagerTransaction(params)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("RehydrateWagerTransaction() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestWagerTransactionRejectsInvalidTerminalFailureWithoutChangingState(t *testing.T) {
	t.Parallel()

	transitions := []struct {
		name string
		call func(*WagerTransaction, WagerTransactionFailureCode, time.Time) error
	}{
		{name: "rejected", call: func(transaction *WagerTransaction, code WagerTransactionFailureCode, now time.Time) error {
			return transaction.MarkRejected(code, now)
		}},
		{name: "failed", call: func(transaction *WagerTransaction, code WagerTransactionFailureCode, now time.Time) error {
			return transaction.MarkFailed(code, now)
		}},
	}

	for _, transition := range transitions {
		transition := transition
		t.Run(transition.name, func(t *testing.T) {
			t.Parallel()

			params := validExternalWagerTransactionParams(t)
			testCases := []struct {
				name    string
				code    WagerTransactionFailureCode
				now     time.Time
				wantErr error
			}{
				{name: "empty failure code", now: params.OccurredAt.Add(time.Second), wantErr: ErrInvalidWagerTransactionFailureCode},
				{name: "zero timestamp", code: WagerTransactionFailureCodeInfrastructureFailure, now: time.Time{}, wantErr: ErrInvalidTimestamp},
				{name: "stale timestamp", code: WagerTransactionFailureCodeInfrastructureFailure, now: params.OccurredAt.Add(-time.Nanosecond), wantErr: ErrStaleTimestamp},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()
					transaction, err := NewExternalWagerTransaction(params)
					if err != nil {
						t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
					}

					err = transition.call(&transaction, testCase.code, testCase.now)
					if !errors.Is(err, testCase.wantErr) {
						t.Fatalf("terminal transition error = %v, want %v", err, testCase.wantErr)
					}
					if transaction.Status() != WagerTransactionStatusPending || transaction.FailureCode() != "" {
						t.Errorf("transaction changed after rejected transition: status=%q failureCode=%q", transaction.Status(), transaction.FailureCode())
					}
					if !transaction.UpdatedAt().Equal(params.OccurredAt) {
						t.Errorf("UpdatedAt() changed to %v, want %v", transaction.UpdatedAt(), params.OccurredAt)
					}
				})
			}
		})
	}
}

func TestWagerTransactionRejectsInvalidReferenceRescheduleWithoutChangingState(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "bet-transaction-456"
	pendingAt := params.OccurredAt.Add(time.Second)
	firstAttemptAt := pendingAt.Add(time.Minute)

	testCases := []struct {
		name          string
		nextAttemptAt time.Time
		now           time.Time
		wantErr       error
	}{
		{name: "zero transition timestamp", nextAttemptAt: firstAttemptAt.Add(time.Minute), now: time.Time{}, wantErr: ErrInvalidTimestamp},
		{name: "zero next attempt", nextAttemptAt: time.Time{}, now: firstAttemptAt, wantErr: ErrInvalidTimestamp},
		{name: "stale transition timestamp", nextAttemptAt: firstAttemptAt.Add(time.Minute), now: pendingAt.Add(-time.Nanosecond), wantErr: ErrStaleTimestamp},
		{name: "next attempt equal to transition", nextAttemptAt: firstAttemptAt, now: firstAttemptAt, wantErr: ErrInvalidReferenceRetrySchedule},
		{name: "next attempt before transition", nextAttemptAt: firstAttemptAt.Add(-time.Nanosecond), now: firstAttemptAt, wantErr: ErrInvalidReferenceRetrySchedule},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			transaction, err := NewExternalWagerTransaction(params)
			if err != nil {
				t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
			}
			if err := transaction.MarkPendingReference(firstAttemptAt, pendingAt); err != nil {
				t.Fatalf("MarkPendingReference() unexpected setup error: %v", err)
			}

			err = transaction.RescheduleReference(testCase.nextAttemptAt, testCase.now)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("RescheduleReference() error = %v, want %v", err, testCase.wantErr)
			}
			if transaction.ReferenceAttempts() != 1 || !transaction.UpdatedAt().Equal(pendingAt) {
				t.Errorf("transaction changed after rejected reschedule: attempts=%d updatedAt=%v", transaction.ReferenceAttempts(), transaction.UpdatedAt())
			}
			gotNextAttempt, ok := transaction.NextReferenceAttemptAt()
			if !ok || !gotNextAttempt.Equal(firstAttemptAt) {
				t.Errorf("NextReferenceAttemptAt() = (%v, %v), want (%v, true)", gotNextAttempt, ok, firstAttemptAt)
			}
		})
	}
}

func TestWagerTransactionRequiresResolvedReferenceBeforeProcessingReversal(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "bet-transaction-456"
	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}
	resultBalance, err := NewMoney("1025.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error: %v", err)
	}

	err = transaction.MarkProcessed(resultBalance, params.OccurredAt.Add(time.Second))
	if !errors.Is(err, ErrReferenceTransactionIDRequired) {
		t.Fatalf("MarkProcessed() error = %v, want ErrReferenceTransactionIDRequired", err)
	}
	if transaction.Status() != WagerTransactionStatusPending || transaction.ReferenceTransactionID() != "" {
		t.Errorf("transaction changed after rejected processing: status=%q reference=%q", transaction.Status(), transaction.ReferenceTransactionID())
	}
}

func TestWagerTransactionResolvesReferenceIdempotently(t *testing.T) {
	t.Parallel()

	params := validExternalWagerTransactionParams(t)
	params.Kind = WagerTransactionKindRefund
	params.ReferenceExternalTransactionID = "bet-transaction-456"
	transaction, err := NewExternalWagerTransaction(params)
	if err != nil {
		t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
	}

	resolvedAt := params.OccurredAt.Add(time.Second)
	if err := transaction.ResolveReference("internal-bet-456", resolvedAt); err != nil {
		t.Fatalf("ResolveReference() unexpected error: %v", err)
	}
	if got := transaction.ReferenceTransactionID(); got != "internal-bet-456" {
		t.Errorf("ReferenceTransactionID() = %q, want internal-bet-456", got)
	}

	if err := transaction.ResolveReference("internal-bet-456", resolvedAt.Add(time.Second)); err != nil {
		t.Fatalf("idempotent ResolveReference() unexpected error: %v", err)
	}
	if !transaction.UpdatedAt().Equal(resolvedAt) {
		t.Errorf("UpdatedAt() changed on idempotent resolution to %v, want %v", transaction.UpdatedAt(), resolvedAt)
	}

	err = transaction.ResolveReference("different-internal-bet", resolvedAt.Add(time.Second))
	if !errors.Is(err, ErrReferenceTransactionAlreadyResolved) {
		t.Fatalf("ResolveReference() error = %v, want ErrReferenceTransactionAlreadyResolved", err)
	}
	if transaction.ReferenceTransactionID() != "internal-bet-456" {
		t.Errorf("ReferenceTransactionID() changed to %q", transaction.ReferenceTransactionID())
	}
}

func TestWagerTransactionRejectsInvalidReferenceResolutionWithoutChangingState(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		kind              WagerTransactionKind
		externalReference string
		internalReference string
		resolveAt         func(time.Time) time.Time
		wantErr           error
	}{
		{name: "unsupported kind", kind: WagerTransactionKindBet, internalReference: "internal-bet", resolveAt: func(now time.Time) time.Time { return now.Add(time.Second) }, wantErr: ErrInvalidWagerTransactionTransition},
		{name: "missing internal reference", kind: WagerTransactionKindRefund, externalReference: "bet-456", internalReference: "", resolveAt: func(now time.Time) time.Time { return now.Add(time.Second) }, wantErr: ErrInvalidReferenceTransactionID},
		{name: "zero timestamp", kind: WagerTransactionKindRefund, externalReference: "bet-456", internalReference: "internal-bet", resolveAt: func(time.Time) time.Time { return time.Time{} }, wantErr: ErrInvalidTimestamp},
		{name: "stale timestamp", kind: WagerTransactionKindRefund, externalReference: "bet-456", internalReference: "internal-bet", resolveAt: func(now time.Time) time.Time { return now.Add(-time.Nanosecond) }, wantErr: ErrStaleTimestamp},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			params := validExternalWagerTransactionParams(t)
			params.Kind = testCase.kind
			params.ReferenceExternalTransactionID = testCase.externalReference
			transaction, err := NewExternalWagerTransaction(params)
			if err != nil {
				t.Fatalf("NewExternalWagerTransaction() unexpected setup error: %v", err)
			}

			err = transaction.ResolveReference(testCase.internalReference, testCase.resolveAt(params.OccurredAt))
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("ResolveReference() error = %v, want %v", err, testCase.wantErr)
			}
			if transaction.ReferenceTransactionID() != "" || !transaction.UpdatedAt().Equal(params.OccurredAt) {
				t.Errorf("transaction changed after rejected resolution: reference=%q updatedAt=%v", transaction.ReferenceTransactionID(), transaction.UpdatedAt())
			}
		})
	}
}

func TestRehydrateWagerTransactionValidatesResolvedReference(t *testing.T) {
	t.Parallel()

	t.Run("restores processed reversal reference", func(t *testing.T) {
		t.Parallel()
		params := validProcessedWagerTransactionRehydrateParams(t)
		params.Kind = WagerTransactionKindRefund
		params.ReferenceExternalTransactionID = "bet-transaction-456"
		params.ReferenceTransactionID = "internal-bet-456"

		transaction, err := RehydrateWagerTransaction(params)
		if err != nil {
			t.Fatalf("RehydrateWagerTransaction() unexpected error: %v", err)
		}
		if transaction.ReferenceTransactionID() != params.ReferenceTransactionID {
			t.Errorf("ReferenceTransactionID() = %q, want %q", transaction.ReferenceTransactionID(), params.ReferenceTransactionID)
		}
	})

	t.Run("rejects processed reversal without resolved reference", func(t *testing.T) {
		t.Parallel()
		params := validProcessedWagerTransactionRehydrateParams(t)
		params.Kind = WagerTransactionKindRefund
		params.ReferenceExternalTransactionID = "bet-transaction-456"

		_, err := RehydrateWagerTransaction(params)
		if !errors.Is(err, ErrReferenceTransactionIDRequired) {
			t.Fatalf("RehydrateWagerTransaction() error = %v, want ErrReferenceTransactionIDRequired", err)
		}
	})

	t.Run("rejects resolved reference for unsupported kind", func(t *testing.T) {
		t.Parallel()
		params := validProcessedWagerTransactionRehydrateParams(t)
		params.ReferenceTransactionID = "internal-bet-456"

		_, err := RehydrateWagerTransaction(params)
		if !errors.Is(err, ErrUnexpectedReferenceTransactionID) {
			t.Fatalf("RehydrateWagerTransaction() error = %v, want ErrUnexpectedReferenceTransactionID", err)
		}
	})
}

func TestNewExternalWagerTransactionRejectsReferenceForUnsupportedKind(t *testing.T) {
	t.Parallel()

	for _, kind := range []WagerTransactionKind{
		WagerTransactionKindBet,
		WagerTransactionKindLoss,
	} {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			params := validExternalWagerTransactionParams(t)
			params.Kind = kind
			params.ReferenceExternalTransactionID = "unexpected-reference"
			if kind == WagerTransactionKindLoss {
				zero, err := NewMoney("0.00", "BRL")
				if err != nil {
					t.Fatalf("NewMoney() unexpected setup error: %v", err)
				}
				params.Money = zero
			}

			_, err := NewExternalWagerTransaction(params)
			if !errors.Is(err, ErrUnexpectedReferenceExternalTransactionID) {
				t.Fatalf("NewExternalWagerTransaction() error = %v, want ErrUnexpectedReferenceExternalTransactionID", err)
			}
		})
	}
}

func validProcessedWagerTransactionRehydrateParams(
	t *testing.T,
) RehydrateWagerTransactionParams {
	t.Helper()

	params := validExternalWagerTransactionParams(t)
	resultBalance, err := NewMoney("975.00", "BRL")
	if err != nil {
		t.Fatalf("NewMoney() unexpected setup error for result balance: %v", err)
	}

	return RehydrateWagerTransactionParams{
		ID:                             params.ID,
		ExternalTransactionID:          params.ExternalTransactionID,
		ProviderID:                     params.ProviderID,
		IdempotencyKey:                 params.IdempotencyKey,
		PayloadHash:                    params.PayloadHash,
		WalletID:                       params.WalletID,
		PlayerID:                       params.PlayerID,
		RoundID:                        params.RoundID,
		GameID:                         params.GameID,
		Kind:                           params.Kind,
		Money:                          params.Money,
		Status:                         WagerTransactionStatusProcessed,
		ReferenceExternalTransactionID: params.ReferenceExternalTransactionID,
		ResultBalance:                  resultBalance,
		HasResultBalance:               true,
		CreatedAt:                      params.OccurredAt,
		UpdatedAt:                      params.OccurredAt.Add(time.Second),
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
