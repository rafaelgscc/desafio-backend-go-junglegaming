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
	EventID       string
	EventType     IntegrationEventType
	AggregateID   string
	CorrelationID string
	CausationID   string
	OccurredAt    time.Time
	Version       int
	Data          any
}

type WagerTransactionProcessedData struct {
	TransactionID         string
	ProviderID            string
	ExternalTransactionID string
	Kind                  domain.WagerTransactionKind
	Money                 domain.Money
	Balance               domain.Money
}

type WagerTransactionRejectedData struct {
	TransactionID         string
	ProviderID            string
	ExternalTransactionID string
	Kind                  domain.WagerTransactionKind
	Money                 domain.Money
	FailureCode           domain.WagerTransactionFailureCode
}

type WalletBalanceChangedData struct {
	WalletID      string
	TransactionID string
	Direction     domain.LedgerDirection
	Money         domain.Money
	BalanceBefore domain.Money
	BalanceAfter  domain.Money
	WalletVersion int64
}

type WagerTransactionPendingReferenceData struct {
	TransactionID                  string
	ProviderID                     string
	ExternalTransactionID          string
	ReferenceExternalTransactionID string
	ReferenceAttempts              int
	NextReferenceAttemptAt         time.Time
}
