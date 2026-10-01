package application

import (
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

type IntegrationEventType string

const (
	IntegrationEventTypeWagerTransactionProcessed        IntegrationEventType = "WagerTransactionProcessed"
	IntegrationEventTypeWagerTransactionRejected         IntegrationEventType = "WagerTransactionRejected"
	IntegrationEventTypeWalletBalanceChanged             IntegrationEventType = "WalletBalanceChanged"
	IntegrationEventTypeWagerTransactionPendingReference IntegrationEventType = "WagerTransactionPendingReference"
)

const IntegrationEventVersion = 1

type IntegrationEvent struct {
	EventID       string               `json:"event_id"`
	EventType     IntegrationEventType `json:"event_type"`
	AggregateID   string               `json:"aggregate_id"`
	CorrelationID string               `json:"correlation_id"`
	CausationID   string               `json:"causation_id,omitempty"`
	OccurredAt    time.Time            `json:"occurred_at"`
	Version       int                  `json:"version"`
	Data          any                  `json:"data"`
}

type WagerTransactionProcessedData struct {
	TransactionID         string                      `json:"transaction_id"`
	ProviderID            string                      `json:"provider_id"`
	ExternalTransactionID string                      `json:"external_transaction_id"`
	Kind                  domain.WagerTransactionKind `json:"kind"`
	Money                 domain.Money                `json:"money"`
	Balance               domain.Money                `json:"balance"`
}

type WagerTransactionRejectedData struct {
	TransactionID         string                             `json:"transaction_id"`
	ProviderID            string                             `json:"provider_id"`
	ExternalTransactionID string                             `json:"external_transaction_id"`
	Kind                  domain.WagerTransactionKind        `json:"kind"`
	Money                 domain.Money                       `json:"money"`
	FailureCode           domain.WagerTransactionFailureCode `json:"failure_code"`
}

type WalletBalanceChangedData struct {
	WalletID      string                 `json:"wallet_id"`
	TransactionID string                 `json:"transaction_id"`
	Direction     domain.LedgerDirection `json:"direction"`
	Money         domain.Money           `json:"money"`
	BalanceBefore domain.Money           `json:"balance_before"`
	BalanceAfter  domain.Money           `json:"balance_after"`
	WalletVersion int64                  `json:"wallet_version"`
}

type WagerTransactionPendingReferenceData struct {
	TransactionID                  string    `json:"transaction_id"`
	ProviderID                     string    `json:"provider_id"`
	ExternalTransactionID          string    `json:"external_transaction_id"`
	ReferenceExternalTransactionID string    `json:"reference_external_transaction_id"`
	ReferenceAttempts              int       `json:"reference_attempts"`
	NextReferenceAttemptAt         time.Time `json:"next_reference_attempt_at"`
}
