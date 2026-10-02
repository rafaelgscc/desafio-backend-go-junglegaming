package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsExposeRequiredSignalsInPrometheusFormat(t *testing.T) {
	metrics := NewMetrics()
	metrics.RecordWagerResult("PROCESSED")
	metrics.RecordDuplicate("http")
	metrics.RecordRetries("outbox", 2)
	metrics.RecordDLQ()
	metrics.RecordConcurrencyConflict()
	metrics.RecordReconciliation(false)
	metrics.ObserveProcessing("http", 250*time.Millisecond)
	metrics.ObserveOutboxLag(time.Unix(10, 0), time.Unix(13, 0))

	response := httptest.NewRecorder()
	metrics.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	for _, expected := range []string{
		`jungle_wager_results_total{status="PROCESSED"} 1`,
		`jungle_duplicates_total{source="http"} 1`,
		`jungle_retries_total{component="outbox"} 2`,
		`jungle_sqs_dlq_total 1`,
		`jungle_concurrency_conflicts_total 1`,
		`jungle_reconciliation_divergences_total 1`,
		`jungle_processing_duration_seconds_count{component="http"} 1`,
		`jungle_processing_duration_seconds_sum{component="http"} 0.25`,
		`jungle_outbox_lag_seconds 3`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("metrics body does not contain %q:\n%s", expected, response.Body)
		}
	}
}

func TestSafeLogValueRemovesLineBreaks(t *testing.T) {
	if got := SafeLogValue("provider\r\nforged"); got != "providerforged" {
		t.Fatalf("SafeLogValue() = %q", got)
	}
}
