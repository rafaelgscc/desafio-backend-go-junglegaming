package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMaxConnections = 10
	defaultMinConnections = 2
)

var (
	ErrDatabaseURLRequired     = errors.New("DATABASE_URL is required")
	ErrInvalidConnectionLimit  = errors.New("invalid database connection limit")
	ErrInvalidConnectionLimits = errors.New(
		"DATABASE_MIN_CONNECTIONS cannot be greater than DATABASE_MAX_CONNECTIONS",
	)
	ErrOIDCIssuerURLRequired = errors.New("OIDC_ISSUER_URL is required")
	ErrOIDCAudienceRequired  = errors.New("OIDC_AUDIENCE is required")
	ErrSQSQueueNameRequired  = errors.New("SQS_QUEUE_NAME is required")
	ErrInvalidWorkerConfig   = errors.New("invalid worker configuration")
)

type PostgresConfig struct {
	DatabaseURL       string
	MaxConnections    int32
	MinConnections    int32
	MaxConnectionIdle time.Duration
	MaxConnectionLife time.Duration
}

type HTTPConfig struct {
	Address           string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type OIDCConfig struct {
	IssuerURL string
	Audience  string
}

type SQSConfig struct {
	Region            string
	Endpoint          string
	QueueName         string
	EventQueueName    string
	ConsumerName      string
	WorkerID          string
	WaitTime          time.Duration
	VisibilityTimeout time.Duration
	MaxMessages       int32
}

type OutboxConfig struct {
	WorkerID      string
	BatchSize     int
	PollInterval  time.Duration
	LeaseDuration time.Duration
}

type ReferenceWorkerConfig struct {
	WorkerID       string
	BatchSize      int
	PollInterval   time.Duration
	LeaseDuration  time.Duration
	RetryBaseDelay time.Duration
	MaxAttempts    int
}

func LoadSQSConfig() (SQSConfig, error) {
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	queueName := strings.TrimSpace(os.Getenv("SQS_QUEUE_NAME"))
	if queueName == "" {
		queueName = "wager-transactions.fifo"
	}
	if !strings.HasSuffix(queueName, ".fifo") {
		return SQSConfig{}, ErrSQSQueueNameRequired
	}
	eventQueueName := strings.TrimSpace(os.Getenv("SQS_EVENT_QUEUE_NAME"))
	if eventQueueName == "" {
		eventQueueName = "integration-events.fifo"
	}
	if !strings.HasSuffix(eventQueueName, ".fifo") {
		return SQSConfig{}, ErrSQSQueueNameRequired
	}
	workerID := strings.TrimSpace(os.Getenv("SQS_WORKER_ID"))
	if workerID == "" {
		hostname, _ := os.Hostname()
		workerID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}
	return SQSConfig{
		Region: region, Endpoint: strings.TrimSpace(os.Getenv("SQS_ENDPOINT")),
		QueueName: queueName, EventQueueName: eventQueueName,
		ConsumerName: "wager-transactions", WorkerID: workerID,
		WaitTime: 10 * time.Second, VisibilityTimeout: 30 * time.Second, MaxMessages: 10,
	}, nil
}

func LoadOutboxConfig() (OutboxConfig, error) {
	workerID := strings.TrimSpace(os.Getenv("OUTBOX_WORKER_ID"))
	if workerID == "" {
		hostname, _ := os.Hostname()
		workerID = fmt.Sprintf("outbox-%s-%d", hostname, os.Getpid())
	}
	return OutboxConfig{
		WorkerID: workerID, BatchSize: 20,
		PollInterval: 500 * time.Millisecond, LeaseDuration: 30 * time.Second,
	}, nil
}

func LoadReferenceWorkerConfig() (ReferenceWorkerConfig, error) {
	workerID := strings.TrimSpace(os.Getenv("REFERENCE_WORKER_ID"))
	if workerID == "" {
		hostname, _ := os.Hostname()
		workerID = fmt.Sprintf("reference-%s-%d", hostname, os.Getpid())
	}
	batchSize, err := positiveIntEnv("REFERENCE_WORKER_BATCH_SIZE", 20)
	if err != nil {
		return ReferenceWorkerConfig{}, err
	}
	if batchSize > 100 {
		return ReferenceWorkerConfig{}, fmt.Errorf(
			"%w: REFERENCE_WORKER_BATCH_SIZE=%d", ErrInvalidWorkerConfig, batchSize,
		)
	}
	maxAttempts, err := positiveIntEnv("REFERENCE_WORKER_MAX_ATTEMPTS", 5)
	if err != nil {
		return ReferenceWorkerConfig{}, err
	}
	pollInterval, err := positiveDurationEnv("REFERENCE_WORKER_POLL_INTERVAL", 500*time.Millisecond)
	if err != nil {
		return ReferenceWorkerConfig{}, err
	}
	leaseDuration, err := positiveDurationEnv("REFERENCE_WORKER_LEASE_DURATION", 30*time.Second)
	if err != nil {
		return ReferenceWorkerConfig{}, err
	}
	retryBaseDelay, err := positiveDurationEnv("REFERENCE_WORKER_RETRY_BASE_DELAY", time.Minute)
	if err != nil {
		return ReferenceWorkerConfig{}, err
	}
	return ReferenceWorkerConfig{
		WorkerID: workerID, BatchSize: batchSize, PollInterval: pollInterval,
		LeaseDuration: leaseDuration, RetryBaseDelay: retryBaseDelay,
		MaxAttempts: maxAttempts,
	}, nil
}

func positiveIntEnv(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%w: %s=%q", ErrInvalidWorkerConfig, name, raw)
	}
	return value, nil
}

func positiveDurationEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%w: %s=%q", ErrInvalidWorkerConfig, name, raw)
	}
	return value, nil
}

func LoadOIDCConfig() (OIDCConfig, error) {
	issuerURL := strings.TrimSpace(os.Getenv("OIDC_ISSUER_URL"))
	if issuerURL == "" {
		return OIDCConfig{}, ErrOIDCIssuerURLRequired
	}
	audience := strings.TrimSpace(os.Getenv("OIDC_AUDIENCE"))
	if audience == "" {
		return OIDCConfig{}, ErrOIDCAudienceRequired
	}
	return OIDCConfig{IssuerURL: issuerURL, Audience: audience}, nil
}

func LoadHTTPConfig() (HTTPConfig, error) {
	address := strings.TrimSpace(os.Getenv("HTTP_ADDRESS"))
	if address == "" {
		address = ":8080"
	}

	return HTTPConfig{
		Address:           address,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}

func LoadPostgresConfig() (PostgresConfig, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return PostgresConfig{}, ErrDatabaseURLRequired
	}

	maxConnections, err := parseConnectionLimit(
		"DATABASE_MAX_CONNECTIONS",
		defaultMaxConnections,
	)
	if err != nil {
		return PostgresConfig{}, err
	}
	minConnections, err := parseConnectionLimit(
		"DATABASE_MIN_CONNECTIONS",
		defaultMinConnections,
	)
	if err != nil {
		return PostgresConfig{}, err
	}
	if minConnections > maxConnections {
		return PostgresConfig{}, ErrInvalidConnectionLimits
	}

	return PostgresConfig{
		DatabaseURL:       databaseURL,
		MaxConnections:    maxConnections,
		MinConnections:    minConnections,
		MaxConnectionIdle: 30 * time.Minute,
		MaxConnectionLife: time.Hour,
	}, nil
}

func parseConnectionLimit(name string, defaultValue int32) (int32, error) {
	rawValue := strings.TrimSpace(os.Getenv(name))
	if rawValue == "" {
		return defaultValue, nil
	}

	value, err := strconv.ParseInt(rawValue, 10, 32)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%w: %s=%q", ErrInvalidConnectionLimit, name, rawValue)
	}

	return int32(value), nil
}
