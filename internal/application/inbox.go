package application

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInboxRepositoryRequired = errors.New("inbox repository is required")
	ErrInboxPayloadConflict    = errors.New("inbox message ID reused with different payload")
	ErrInboxLeaseLost          = errors.New("inbox message lease lost")
)

type InboxMessage struct {
	ConsumerName string
	MessageID    string
	EventID      string
	EventType    string
	PayloadHash  string
	Payload      []byte
	ReceivedAt   time.Time
}

type InboxClaimStatus string

const (
	InboxClaimed          InboxClaimStatus = "CLAIMED"
	InboxAlreadyProcessed InboxClaimStatus = "ALREADY_PROCESSED"
	InboxBusy             InboxClaimStatus = "BUSY"
)

type InboxRepository interface {
	Claim(
		context.Context,
		InboxMessage,
		string,
		time.Time,
	) (InboxClaimStatus, error)
	Complete(context.Context, string, string, string, time.Time) error
	Fail(context.Context, string, string, string, time.Time, string) error
}

type InboxCompletion struct {
	ConsumerName string
	MessageID    string
	LeaseOwner   string
	ProcessedAt  time.Time
}
