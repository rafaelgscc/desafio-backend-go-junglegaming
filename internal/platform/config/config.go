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
