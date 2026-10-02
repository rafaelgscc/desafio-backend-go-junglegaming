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
	EventID       string               `json:"eventId"`
	EventType     IntegrationEventType `json:"eventType"`
	AggregateID   string               `json:"aggregateId"`
	CorrelationID string               `json:"correlationId"`
	CausationID   string               `json:"causationId,omitempty"`
	OccurredAt    time.Time            `json:"occurredAt"`
	Version       int                  `json:"version"`
	Data          any                  `json:"data"`
}

type WagerTransactionProcessedData struct {
	TransactionID         string                      `json:"transactionId"`
	ProviderID            string                      `json:"providerId"`
	ExternalTransactionID string                      `json:"externalTransactionId"`
	Kind                  domain.WagerTransactionKind `json:"kind"`
	Money                 domain.Money                `json:"money"`
	Balance               domain.Money                `json:"balance"`
}

type WagerTransactionRejectedData struct {
	TransactionID         string                             `json:"transactionId"`
	ProviderID            string                             `json:"providerId"`
	ExternalTransactionID string                             `json:"externalTransactionId"`
	Kind                  domain.WagerTransactionKind        `json:"kind"`
	Money                 domain.Money                       `json:"money"`
	FailureCode           domain.WagerTransactionFailureCode `json:"failureCode"`
}

type WalletBalanceChangedData struct {
	WalletID      string                 `json:"walletId"`
	TransactionID string                 `json:"transactionId"`
	Direction     domain.LedgerDirection `json:"direction"`
	Money         domain.Money           `json:"money"`
	BalanceBefore domain.Money           `json:"balanceBefore"`
	BalanceAfter  domain.Money           `json:"balanceAfter"`
	WalletVersion int64                  `json:"walletVersion"`
}

type WagerTransactionPendingReferenceData struct {
	TransactionID                  string    `json:"transactionId"`
	ProviderID                     string    `json:"providerId"`
	ExternalTransactionID          string    `json:"externalTransactionId"`
	ReferenceExternalTransactionID string    `json:"referenceExternalTransactionId"`
	ReferenceAttempts              int       `json:"referenceAttempts"`
	NextReferenceAttemptAt         time.Time `json:"nextReferenceAttemptAt"`
}
