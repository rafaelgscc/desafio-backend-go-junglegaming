package httpadapter

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"
)

var (
	ErrOpenWalletExecutorRequired              = errors.New("open wallet executor is required")
	ErrExecuteWagerTransactionExecutorRequired = errors.New(
		"execute wager transaction executor is required",
	)
	ErrIDGeneratorRequired = errors.New("ID generator is required")
	ErrClockRequired       = errors.New("clock is required")
)

type IDGenerator interface {
	NewID() (string, error)
}

type Clock interface {
	Now() time.Time
}

type RandomIDGenerator struct {
	random io.Reader
}

func NewRandomIDGenerator() IDGenerator {
	return &RandomIDGenerator{random: rand.Reader}
}

func (generator *RandomIDGenerator) NewID() (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(generator.random, value[:]); err != nil {
		return "", fmt.Errorf("generate random ID: %w", err)
	}

	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	), nil
}

type SystemClock struct{}

func NewSystemClock() Clock {
	return SystemClock{}
}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}
