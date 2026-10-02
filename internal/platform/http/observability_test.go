package httpadapter

import "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/observability"

func newTestMetrics() *observability.Metrics {
	return observability.NewMetrics()
}
