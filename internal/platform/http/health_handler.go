package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"time"
)

const readinessTimeout = 2 * time.Second

var ErrDatabaseHealthCheckerRequired = errors.New("database health checker is required")

type DatabaseHealthChecker interface {
	Ping(context.Context) error
}

type HealthHandler struct {
	database DatabaseHealthChecker
}

func NewHealthHandler(database DatabaseHealthChecker) (*HealthHandler, error) {
	if database == nil {
		return nil, ErrDatabaseHealthCheckerRequired
	}

	return &HealthHandler{database: database}, nil
}

func (handler *HealthHandler) Live(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (handler *HealthHandler) Ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), readinessTimeout)
	defer cancel()

	if err := handler.database.Ping(ctx); err != nil {
		writeJSON(
			response,
			http.StatusServiceUnavailable,
			map[string]string{"status": "unavailable"},
		)
		return
	}

	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}
