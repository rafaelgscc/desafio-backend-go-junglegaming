package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidWagerTransactionID              = errors.New("invalid wager transaction ID")
	ErrInvalidExternalTransactionID           = errors.New("invalid external transaction ID")
	ErrInvalidProviderID                      = errors.New("invalid provider ID")
	ErrInvalidIdempotencyKey                  = errors.New("invalid idempotency key")
	ErrInvalidPayloadHash                     = errors.New("invalid payload hash")
	ErrInvalidRoundID                         = errors.New("invalid round ID")
	ErrInvalidGameID                          = errors.New("invalid game ID")
	ErrInvalidWagerTransactionKind            = errors.New("invalid wager transaction kind")
	ErrLossAmountMustBeZero                   = errors.New("LOSS amount must be zero")
	ErrReferenceExternalTransactionIDRequired = errors.New(
		"reference external transaction ID is required",
	)
	ErrUnexpectedReferenceExternalTransactionID = errors.New(
		"reference external transaction ID is not allowed for this transaction kind",
	)
	ErrInvalidWagerTransactionTransition = errors.New(
		"invalid wager transaction state transition",
	)
	ErrInvalidReferenceRetrySchedule = errors.New(
		"next reference attempt must be after transition time",
	)
	ErrReferenceRetryNotDue = errors.New(
		"reference retry is not due yet",
	)
	ErrInvalidWagerTransactionFailureCode = errors.New(
		"invalid wager transaction failure code",
	)
	ErrWagerTransactionResultBalanceRequired = errors.New(
		"processed wager transaction requires a result balance",
	)
	ErrInvalidReferenceRetryState = errors.New(
		"invalid reference retry state",
	)
	ErrUnexpectedWagerTransactionResultBalance = errors.New(
		"result balance is only allowed for processed wager transactions",
	)
	ErrUnexpectedWagerTransactionFailureCode = errors.New(
		"failure code is only allowed for rejected or failed wager transactions",
	)
	ErrUnexpectedExternalWagerTransactionMetadata = errors.New(
		"external wager transaction metadata is not allowed for opening",
	)
	ErrOpeningResultBalanceMismatch = errors.New(
		"opening result balance must equal opening money",
	)
	ErrInvalidReferenceTransactionID = errors.New(
		"invalid reference transaction ID",
	)
	ErrReferenceTransactionIDRequired = errors.New(
		"resolved reference transaction ID is required",
	)
	ErrReferenceTransactionAlreadyResolved = errors.New(
		"reference transaction is already resolved with another ID",
	)
	ErrUnexpectedReferenceTransactionID = errors.New(
		"resolved reference transaction ID is not allowed for this transaction kind",
	)
)

type WagerTransactionKind string

const (
	WagerTransactionKindOpening  WagerTransactionKind = "OPENING"
	WagerTransactionKindBet      WagerTransactionKind = "BET"
	WagerTransactionKindWin      WagerTransactionKind = "WIN"
	WagerTransactionKindLoss     WagerTransactionKind = "LOSS"
	WagerTransactionKindRefund   WagerTransactionKind = "REFUND"
	WagerTransactionKindRollback WagerTransactionKind = "ROLLBACK"
)

type WagerTransactionStatus string

const (
	WagerTransactionStatusPending          WagerTransactionStatus = "PENDING"
	WagerTransactionStatusPendingReference WagerTransactionStatus = "PENDING_REFERENCE"
	WagerTransactionStatusProcessed        WagerTransactionStatus = "PROCESSED"
	WagerTransactionStatusRejected         WagerTransactionStatus = "REJECTED"
	WagerTransactionStatusFailed           WagerTransactionStatus = "FAILED"
)

type WagerTransactionFailureCode string

const (
	WagerTransactionFailureCodeReferenceNotFound         WagerTransactionFailureCode = "REFERENCE_NOT_FOUND"
	WagerTransactionFailureCodeInfrastructureFailure     WagerTransactionFailureCode = "INFRASTRUCTURE_FAILURE"
	WagerTransactionFailureCodeInsufficientFunds         WagerTransactionFailureCode = "INSUFFICIENT_FUNDS"
	WagerTransactionFailureCodeReversalInsufficientFunds WagerTransactionFailureCode = "REVERSAL_INSUFFICIENT_FUNDS"
	WagerTransactionFailureCodeReferenceNotProcessable   WagerTransactionFailureCode = "REFERENCE_NOT_PROCESSABLE"
	WagerTransactionFailureCodeReferenceMismatch         WagerTransactionFailureCode = "REFERENCE_MISMATCH"
	WagerTransactionFailureCodeDuplicateReversal         WagerTransactionFailureCode = "DUPLICATE_REVERSAL"
)

type NewExternalWagerTransactionParams struct {
	ID                             string
	ExternalTransactionID          string
	ProviderID                     string
	IdempotencyKey                 string
	PayloadHash                    string
	WalletID                       string
	PlayerID                       string
	RoundID                        string
	GameID                         string
	Kind                           WagerTransactionKind
	Money                          Money
	ReferenceExternalTransactionID string
	OccurredAt                     time.Time
}

type NewOpeningWagerTransactionParams struct {
	ID         string
	WalletID   string
	PlayerID   string
	Money      Money
	OccurredAt time.Time
}

type WagerTransaction struct {
	id                             string
	externalTransactionID          string
	providerID                     string
	idempotencyKey                 string
	payloadHash                    string
	walletID                       string
	playerID                       string
	roundID                        string
	gameID                         string
	kind                           WagerTransactionKind
	money                          Money
	status                         WagerTransactionStatus
	failureCode                    WagerTransactionFailureCode
	referenceExternalTransactionID string
	referenceTransactionID         string
	resultBalance                  Money
	hasResultBalance               bool
	referenceAttempts              int
	nextReferenceAttemptAt         time.Time
	hasNextReferenceAttempt        bool
	createdAt                      time.Time
	updatedAt                      time.Time
}

type RehydrateWagerTransactionParams struct {
	ID                             string
	ExternalTransactionID          string
	ProviderID                     string
	IdempotencyKey                 string
	PayloadHash                    string
	WalletID                       string
	PlayerID                       string
	RoundID                        string
	GameID                         string
	Kind                           WagerTransactionKind
	Money                          Money
	Status                         WagerTransactionStatus
	ReferenceExternalTransactionID string
	ReferenceTransactionID         string
	ResultBalance                  Money
	HasResultBalance               bool
	FailureCode                    WagerTransactionFailureCode
	ReferenceAttempts              int
	NextReferenceAttemptAt         time.Time
	HasNextReferenceAttempt        bool
	CreatedAt                      time.Time
	UpdatedAt                      time.Time
}

func NewExternalWagerTransaction(
	params NewExternalWagerTransactionParams,
) (WagerTransaction, error) {
	if err := validateNewExternalWagerTransactionParams(params); err != nil {
		return WagerTransaction{}, err
	}

	occurredAt := params.OccurredAt.UTC()

	return WagerTransaction{
		id:                             params.ID,
		externalTransactionID:          params.ExternalTransactionID,
		providerID:                     params.ProviderID,
		idempotencyKey:                 params.IdempotencyKey,
		payloadHash:                    params.PayloadHash,
		walletID:                       params.WalletID,
		playerID:                       params.PlayerID,
		roundID:                        params.RoundID,
		gameID:                         params.GameID,
		kind:                           params.Kind,
		money:                          params.Money,
		status:                         WagerTransactionStatusPending,
		referenceExternalTransactionID: params.ReferenceExternalTransactionID,
		createdAt:                      occurredAt,
		updatedAt:                      occurredAt,
	}, nil
}

func NewOpeningWagerTransaction(
	params NewOpeningWagerTransactionParams,
) (WagerTransaction, error) {
	if err := validateNewOpeningWagerTransactionParams(params); err != nil {
		return WagerTransaction{}, err
	}

	occurredAt := params.OccurredAt.UTC()
	return WagerTransaction{
		id:               params.ID,
		walletID:         params.WalletID,
		playerID:         params.PlayerID,
		kind:             WagerTransactionKindOpening,
		money:            params.Money,
		status:           WagerTransactionStatusProcessed,
		resultBalance:    params.Money,
		hasResultBalance: true,
		createdAt:        occurredAt,
		updatedAt:        occurredAt,
	}, nil
}

func RehydrateWagerTransaction(
	params RehydrateWagerTransactionParams,
) (WagerTransaction, error) {
	var (
		transaction WagerTransaction
		err         error
	)

	if params.Kind == WagerTransactionKindOpening {
		if hasExternalWagerTransactionMetadata(params) {
			return WagerTransaction{}, ErrUnexpectedExternalWagerTransactionMetadata
		}

		transaction, err = NewOpeningWagerTransaction(
			NewOpeningWagerTransactionParams{
				ID:         params.ID,
				WalletID:   params.WalletID,
				PlayerID:   params.PlayerID,
				Money:      params.Money,
				OccurredAt: params.CreatedAt,
			},
		)
	} else {
		transaction, err = NewExternalWagerTransaction(NewExternalWagerTransactionParams{
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
			ReferenceExternalTransactionID: params.ReferenceExternalTransactionID,
			OccurredAt:                     params.CreatedAt,
		})
	}
	if err != nil {
		return WagerTransaction{}, err
	}

	if params.UpdatedAt.IsZero() {
		return WagerTransaction{}, ErrInvalidTimestamp
	}

	updatedAt := params.UpdatedAt.UTC()
	if updatedAt.Before(transaction.createdAt) {
		return WagerTransaction{}, ErrStaleTimestamp
	}

	if err := validateRehydratedWagerTransactionState(params, updatedAt); err != nil {
		return WagerTransaction{}, err
	}

	transaction.status = params.Status
	transaction.failureCode = params.FailureCode
	transaction.referenceTransactionID = params.ReferenceTransactionID
	transaction.referenceAttempts = params.ReferenceAttempts
	transaction.updatedAt = updatedAt

	if params.HasResultBalance {
		transaction.resultBalance = params.ResultBalance
		transaction.hasResultBalance = true
	}

	if params.HasNextReferenceAttempt {
		transaction.nextReferenceAttemptAt = params.NextReferenceAttemptAt.UTC()
		transaction.hasNextReferenceAttempt = true
	}

	return transaction, nil
}

func (t WagerTransaction) ID() string {
	return t.id
}

func (t WagerTransaction) ExternalTransactionID() string {
	return t.externalTransactionID
}

func (t WagerTransaction) ProviderID() string {
	return t.providerID
}

func (t WagerTransaction) IdempotencyKey() string {
	return t.idempotencyKey
}

func (t WagerTransaction) PayloadHash() string {
	return t.payloadHash
}

func (t WagerTransaction) WalletID() string {
	return t.walletID
}

func (t WagerTransaction) PlayerID() string {
	return t.playerID
}

func (t WagerTransaction) RoundID() string {
	return t.roundID
}

func (t WagerTransaction) GameID() string {
	return t.gameID
}

func (t WagerTransaction) Kind() WagerTransactionKind {
	return t.kind
}

func (t WagerTransaction) Money() Money {
	return t.money
}

func (t WagerTransaction) Status() WagerTransactionStatus {
	return t.status
}

func (t WagerTransaction) CreatedAt() time.Time {
	return t.createdAt
}

func (t WagerTransaction) UpdatedAt() time.Time {
	return t.updatedAt
}

func (t WagerTransaction) ReferenceExternalTransactionID() string {
	return t.referenceExternalTransactionID
}

func (t WagerTransaction) ReferenceTransactionID() string {
	return t.referenceTransactionID
}

func (t WagerTransaction) ResultBalance() (Money, bool) {
	return t.resultBalance, t.hasResultBalance
}

func (t WagerTransaction) FailureCode() WagerTransactionFailureCode {
	return t.failureCode
}

func (t WagerTransaction) ReferenceAttempts() int {
	return t.referenceAttempts
}

func (t WagerTransaction) NextReferenceAttemptAt() (time.Time, bool) {
	return t.nextReferenceAttemptAt, t.hasNextReferenceAttempt
}

func (t *WagerTransaction) MarkProcessed(
	resultBalance Money,
	now time.Time,
) error {
	if !t.hasActiveStatus() {
		return ErrInvalidWagerTransactionTransition
	}
	if t.kind.isReversal() && t.referenceTransactionID == "" {
		return ErrReferenceTransactionIDRequired
	}

	if err := validateWagerResultBalance(resultBalance, t.money); err != nil {
		return err
	}

	now, err := normalizeTransitionTime(now, t.updatedAt)
	if err != nil {
		return err
	}

	t.status = WagerTransactionStatusProcessed
	t.resultBalance = resultBalance
	t.hasResultBalance = true
	t.clearReferenceSchedule()
	t.updatedAt = now

	return nil
}

func (t *WagerTransaction) ResolveReference(
	referenceTransactionID string,
	now time.Time,
) error {
	if !t.hasActiveStatus() ||
		!t.kind.supportsReference() ||
		t.referenceExternalTransactionID == "" {
		return ErrInvalidWagerTransactionTransition
	}
	if referenceTransactionID == "" {
		return ErrInvalidReferenceTransactionID
	}

	now, err := normalizeTransitionTime(now, t.updatedAt)
	if err != nil {
		return err
	}

	if t.referenceTransactionID != "" {
		if t.referenceTransactionID == referenceTransactionID {
			return nil
		}
		return ErrReferenceTransactionAlreadyResolved
	}

	t.referenceTransactionID = referenceTransactionID
	t.updatedAt = now
	return nil
}

func (t *WagerTransaction) MarkPendingReference(
	nextAttemptAt time.Time,
	now time.Time,
) error {
	if t.status != WagerTransactionStatusPending {
		return ErrInvalidWagerTransactionTransition
	}

	if !t.kind.isReversal() {
		return ErrInvalidWagerTransactionTransition
	}

	if now.IsZero() || nextAttemptAt.IsZero() {
		return ErrInvalidTimestamp
	}

	now = now.UTC()
	nextAttemptAt = nextAttemptAt.UTC()

	if now.Before(t.updatedAt) {
		return ErrStaleTimestamp
	}

	if !nextAttemptAt.After(now) {
		return ErrInvalidReferenceRetrySchedule
	}

	t.status = WagerTransactionStatusPendingReference
	t.referenceAttempts = 1
	t.nextReferenceAttemptAt = nextAttemptAt
	t.hasNextReferenceAttempt = true
	t.updatedAt = now

	return nil
}

func (t *WagerTransaction) RescheduleReference(
	nextAttemptAt time.Time,
	now time.Time,
) error {
	if t.status != WagerTransactionStatusPendingReference {
		return ErrInvalidWagerTransactionTransition
	}

	if now.IsZero() || nextAttemptAt.IsZero() {
		return ErrInvalidTimestamp
	}

	now = now.UTC()
	nextAttemptAt = nextAttemptAt.UTC()

	if now.Before(t.updatedAt) {
		return ErrStaleTimestamp
	}

	if t.hasNextReferenceAttempt &&
		now.Before(t.nextReferenceAttemptAt) {
		return ErrReferenceRetryNotDue
	}

	if !nextAttemptAt.After(now) {
		return ErrInvalidReferenceRetrySchedule
	}

	t.referenceAttempts++
	t.nextReferenceAttemptAt = nextAttemptAt
	t.hasNextReferenceAttempt = true
	t.updatedAt = now

	return nil
}

func (t *WagerTransaction) MarkRejected(
	failureCode WagerTransactionFailureCode,
	now time.Time,
) error {
	return t.markTerminalFailure(
		WagerTransactionStatusRejected,
		failureCode,
		now,
	)
}

func (t *WagerTransaction) MarkFailed(
	failureCode WagerTransactionFailureCode,
	now time.Time,
) error {
	return t.markTerminalFailure(
		WagerTransactionStatusFailed,
		failureCode,
		now,
	)
}

func (t WagerTransaction) hasActiveStatus() bool {
	return t.status == WagerTransactionStatusPending ||
		t.status == WagerTransactionStatusPendingReference
}

func (t *WagerTransaction) markTerminalFailure(
	status WagerTransactionStatus,
	failureCode WagerTransactionFailureCode,
	now time.Time,
) error {
	if !t.hasActiveStatus() {
		return ErrInvalidWagerTransactionTransition
	}

	if failureCode == "" {
		return ErrInvalidWagerTransactionFailureCode
	}

	now, err := normalizeTransitionTime(now, t.updatedAt)
	if err != nil {
		return err
	}

	t.status = status
	t.failureCode = failureCode
	t.clearReferenceSchedule()
	t.updatedAt = now

	return nil
}

func (t *WagerTransaction) clearReferenceSchedule() {
	t.nextReferenceAttemptAt = time.Time{}
	t.hasNextReferenceAttempt = false
}

func normalizeTransitionTime(now, current time.Time) (time.Time, error) {
	if now.IsZero() {
		return time.Time{}, ErrInvalidTimestamp
	}

	now = now.UTC()
	if now.Before(current) {
		return time.Time{}, ErrStaleTimestamp
	}

	return now, nil
}

func validateWagerResultBalance(resultBalance, transactionMoney Money) error {
	if err := resultBalance.validate(); err != nil {
		return err
	}

	if _, err := resultBalance.Compare(transactionMoney); err != nil {
		return err
	}

	zero, err := ZeroMoney(resultBalance.Currency())
	if err != nil {
		return err
	}

	comparison, err := resultBalance.Compare(zero)
	if err != nil {
		return err
	}

	if comparison < 0 {
		return ErrNegativeBalance
	}

	return nil
}

func validateRehydratedWagerTransactionState(
	params RehydrateWagerTransactionParams,
	updatedAt time.Time,
) error {
	switch params.Status {
	case WagerTransactionStatusPending,
		WagerTransactionStatusPendingReference,
		WagerTransactionStatusProcessed,
		WagerTransactionStatusRejected,
		WagerTransactionStatusFailed:
	default:
		return ErrInvalidWagerTransactionTransition
	}

	if params.Kind == WagerTransactionKindOpening &&
		params.Status != WagerTransactionStatusProcessed {
		return ErrInvalidWagerTransactionTransition
	}

	if params.ReferenceAttempts < 0 {
		return ErrInvalidReferenceRetryState
	}

	if params.ReferenceTransactionID != "" &&
		(!params.Kind.supportsReference() || params.ReferenceExternalTransactionID == "") {
		return ErrUnexpectedReferenceTransactionID
	}
	if params.Status == WagerTransactionStatusProcessed &&
		params.Kind.isReversal() &&
		params.ReferenceTransactionID == "" {
		return ErrReferenceTransactionIDRequired
	}

	if params.Status == WagerTransactionStatusProcessed {
		if !params.HasResultBalance {
			return ErrWagerTransactionResultBalanceRequired
		}
		if err := validateWagerResultBalance(params.ResultBalance, params.Money); err != nil {
			return err
		}
		if params.Kind == WagerTransactionKindOpening {
			comparison, err := params.ResultBalance.Compare(params.Money)
			if err != nil {
				return err
			}
			if comparison != 0 {
				return ErrOpeningResultBalanceMismatch
			}
		}
	} else if params.HasResultBalance {
		return ErrUnexpectedWagerTransactionResultBalance
	}

	switch params.Status {
	case WagerTransactionStatusRejected, WagerTransactionStatusFailed:
		if params.FailureCode == "" {
			return ErrInvalidWagerTransactionFailureCode
		}
	default:
		if params.FailureCode != "" {
			return ErrUnexpectedWagerTransactionFailureCode
		}
	}

	if params.Status == WagerTransactionStatusPendingReference {
		nextAttemptAt := params.NextReferenceAttemptAt.UTC()
		if !params.Kind.isReversal() ||
			params.ReferenceAttempts <= 0 ||
			!params.HasNextReferenceAttempt ||
			params.NextReferenceAttemptAt.IsZero() ||
			!nextAttemptAt.After(updatedAt) {
			return ErrInvalidReferenceRetryState
		}
		return nil
	}

	if params.HasNextReferenceAttempt || !params.NextReferenceAttemptAt.IsZero() {
		return ErrInvalidReferenceRetryState
	}

	if params.Status == WagerTransactionStatusPending && params.ReferenceAttempts != 0 {
		return ErrInvalidReferenceRetryState
	}

	return nil
}

func validateNewExternalWagerTransactionParams(
	params NewExternalWagerTransactionParams,
) error {
	switch {
	case params.ID == "":
		return ErrInvalidWagerTransactionID
	case params.ExternalTransactionID == "":
		return ErrInvalidExternalTransactionID
	case params.ProviderID == "":
		return ErrInvalidProviderID
	case params.IdempotencyKey == "":
		return ErrInvalidIdempotencyKey
	case params.PayloadHash == "":
		return ErrInvalidPayloadHash
	case params.WalletID == "":
		return ErrInvalidWalletID
	case params.PlayerID == "":
		return ErrInvalidPlayerID
	case params.RoundID == "":
		return ErrInvalidRoundID
	case params.GameID == "":
		return ErrInvalidGameID
	}

	switch params.Kind {
	case WagerTransactionKindBet,
		WagerTransactionKindWin,
		WagerTransactionKindLoss,
		WagerTransactionKindRefund,
		WagerTransactionKindRollback:
	default:
		return ErrInvalidWagerTransactionKind
	}

	if err := validateExternalWagerMoney(params.Kind, params.Money); err != nil {
		return err
	}

	switch params.Kind {
	case WagerTransactionKindRefund,
		WagerTransactionKindRollback:
		if params.ReferenceExternalTransactionID == "" {
			return ErrReferenceExternalTransactionIDRequired
		}
	case WagerTransactionKindBet,
		WagerTransactionKindLoss:
		if params.ReferenceExternalTransactionID != "" {
			return ErrUnexpectedReferenceExternalTransactionID
		}
	}

	if params.OccurredAt.IsZero() {
		return ErrInvalidTimestamp
	}

	return nil
}

func validateNewOpeningWagerTransactionParams(
	params NewOpeningWagerTransactionParams,
) error {
	switch {
	case params.ID == "":
		return ErrInvalidWagerTransactionID
	case params.WalletID == "":
		return ErrInvalidWalletID
	case params.PlayerID == "":
		return ErrInvalidPlayerID
	case params.OccurredAt.IsZero():
		return ErrInvalidTimestamp
	}

	if err := params.Money.validate(); err != nil {
		return err
	}

	zero, err := ZeroMoney(params.Money.Currency())
	if err != nil {
		return err
	}
	comparison, err := params.Money.Compare(zero)
	if err != nil {
		return err
	}
	if comparison <= 0 {
		return ErrNonPositiveAmount
	}

	return nil
}

func hasExternalWagerTransactionMetadata(params RehydrateWagerTransactionParams) bool {
	return params.ExternalTransactionID != "" ||
		params.ProviderID != "" ||
		params.IdempotencyKey != "" ||
		params.PayloadHash != "" ||
		params.RoundID != "" ||
		params.GameID != "" ||
		params.ReferenceExternalTransactionID != ""
}

func validateExternalWagerMoney(
	kind WagerTransactionKind,
	money Money,
) error {
	if err := money.validate(); err != nil {
		return err
	}

	zero, err := ZeroMoney(money.Currency())
	if err != nil {
		return err
	}

	comparison, err := money.Compare(zero)
	if err != nil {
		return err
	}

	switch kind {
	case WagerTransactionKindLoss:
		if comparison != 0 {
			return ErrLossAmountMustBeZero
		}

	case WagerTransactionKindBet,
		WagerTransactionKindWin,
		WagerTransactionKindRefund,
		WagerTransactionKindRollback:
		if comparison <= 0 {
			return ErrNonPositiveAmount
		}
	}

	return nil
}

func (kind WagerTransactionKind) isReversal() bool {
	return kind == WagerTransactionKindRefund ||
		kind == WagerTransactionKindRollback
}

func (kind WagerTransactionKind) supportsReference() bool {
	return kind.isReversal() || kind == WagerTransactionKindWin
}
