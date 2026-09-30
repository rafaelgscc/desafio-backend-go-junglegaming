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
	WagerTransactionStatusPending WagerTransactionStatus = "PENDING"
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
	referenceExternalTransactionID string
	createdAt                      time.Time
	updatedAt                      time.Time
}

func NewExternalWagerTransaction(
	params NewExternalWagerTransactionParams,
) (WagerTransaction, error) {
	if params.ID == "" {
		return WagerTransaction{}, ErrInvalidWagerTransactionID
	}

	if params.ExternalTransactionID == "" {
		return WagerTransaction{}, ErrInvalidExternalTransactionID
	}

	if params.ProviderID == "" {
		return WagerTransaction{}, ErrInvalidProviderID
	}

	if params.IdempotencyKey == "" {
		return WagerTransaction{}, ErrInvalidIdempotencyKey
	}

	if params.PayloadHash == "" {
		return WagerTransaction{}, ErrInvalidPayloadHash
	}

	if params.WalletID == "" {
		return WagerTransaction{}, ErrInvalidWalletID
	}

	if params.PlayerID == "" {
		return WagerTransaction{}, ErrInvalidPlayerID
	}

	if params.RoundID == "" {
		return WagerTransaction{}, ErrInvalidRoundID
	}

	if params.GameID == "" {
		return WagerTransaction{}, ErrInvalidGameID
	}

	switch params.Kind {
	case WagerTransactionKindBet,
		WagerTransactionKindWin,
		WagerTransactionKindLoss,
		WagerTransactionKindRefund,
		WagerTransactionKindRollback:
	default:
		return WagerTransaction{}, ErrInvalidWagerTransactionKind
	}

	if err := validateExternalWagerMoney(params.Kind, params.Money); err != nil {
		return WagerTransaction{}, err
	}

	switch params.Kind {
	case WagerTransactionKindRefund,
		WagerTransactionKindRollback:
		if params.ReferenceExternalTransactionID == "" {
			return WagerTransaction{}, ErrReferenceExternalTransactionIDRequired
		}
	}

	if params.OccurredAt.IsZero() {
		return WagerTransaction{}, ErrInvalidTimestamp
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

func (t WagerTransaction) ReferenceExternalTransactionID() string {
	return t.referenceExternalTransactionID
}
