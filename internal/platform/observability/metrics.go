package observability

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrMetricsRequired = errors.New("metrics registry is required")

// Metrics is a small in-process Prometheus registry. Labels are restricted to
// values controlled by the application, avoiding unbounded IDs as labels.
type Metrics struct {
	mu sync.RWMutex

	wagerResults             map[string]uint64
	duplicates               map[string]uint64
	retries                  map[string]uint64
	dlqMessages              uint64
	concurrencyConflicts     uint64
	reconciliationDivergence uint64
	processingCount          map[string]uint64
	processingSeconds        map[string]float64
	outboxLagSeconds         float64
}

func NewMetrics() *Metrics {
	return &Metrics{
		wagerResults:      make(map[string]uint64),
		duplicates:        make(map[string]uint64),
		retries:           make(map[string]uint64),
		processingCount:   make(map[string]uint64),
		processingSeconds: make(map[string]float64),
	}
}

func (metrics *Metrics) RecordWagerResult(status string) {
	metrics.mu.Lock()
	metrics.wagerResults[status]++
	metrics.mu.Unlock()
}

func (metrics *Metrics) RecordDuplicate(source string) {
	metrics.mu.Lock()
	metrics.duplicates[source]++
	metrics.mu.Unlock()
}

func (metrics *Metrics) RecordRetries(component string, count int) {
	if count < 1 {
		return
	}
	metrics.mu.Lock()
	metrics.retries[component] += uint64(count)
	metrics.mu.Unlock()
}

func (metrics *Metrics) RecordDLQ() {
	metrics.mu.Lock()
	metrics.dlqMessages++
	metrics.mu.Unlock()
}

func (metrics *Metrics) RecordConcurrencyConflict() {
	metrics.mu.Lock()
	metrics.concurrencyConflicts++
	metrics.mu.Unlock()
}

func (metrics *Metrics) RecordReconciliation(consistent bool) {
	if consistent {
		return
	}
	metrics.mu.Lock()
	metrics.reconciliationDivergence++
	metrics.mu.Unlock()
}

func (metrics *Metrics) ObserveProcessing(component string, duration time.Duration) {
	metrics.mu.Lock()
	metrics.processingCount[component]++
	metrics.processingSeconds[component] += duration.Seconds()
	metrics.mu.Unlock()
}

func (metrics *Metrics) ObserveOutboxLag(occurredAt, observedAt time.Time) {
	lag := observedAt.Sub(occurredAt).Seconds()
	if lag < 0 {
		lag = 0
	}
	metrics.mu.Lock()
	metrics.outboxLagSeconds = lag
	metrics.mu.Unlock()
}

func (metrics *Metrics) ServeHTTP(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	metrics.WritePrometheus(response)
}

func (metrics *Metrics) WritePrometheus(destination io.Writer) {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	writeMapMetric(destination, "jungle_wager_results_total", "status", metrics.wagerResults)
	writeMapMetric(destination, "jungle_duplicates_total", "source", metrics.duplicates)
	writeMapMetric(destination, "jungle_retries_total", "component", metrics.retries)
	fmt.Fprintf(destination, "jungle_sqs_dlq_total %d\n", metrics.dlqMessages)
	fmt.Fprintf(destination, "jungle_concurrency_conflicts_total %d\n", metrics.concurrencyConflicts)
	fmt.Fprintf(destination, "jungle_reconciliation_divergences_total %d\n", metrics.reconciliationDivergence)
	writeMapMetric(destination, "jungle_processing_duration_seconds_count", "component", metrics.processingCount)
	writeFloatMapMetric(destination, "jungle_processing_duration_seconds_sum", "component", metrics.processingSeconds)
	fmt.Fprintf(destination, "jungle_outbox_lag_seconds %s\n", strconv.FormatFloat(metrics.outboxLagSeconds, 'f', -1, 64))
}

func writeMapMetric(destination io.Writer, name, label string, values map[string]uint64) {
	for _, value := range sortedKeys(values) {
		fmt.Fprintf(destination, "%s{%s=%q} %d\n", name, label, value, values[value])
	}
}

func writeFloatMapMetric(destination io.Writer, name, label string, values map[string]float64) {
	for _, value := range sortedKeys(values) {
		fmt.Fprintf(destination, "%s{%s=%q} %s\n", name, label, value,
			strconv.FormatFloat(values[value], 'f', -1, 64))
	}
}

func sortedKeys[T ~uint64 | ~float64](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func SafeLogValue(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r", ""), "\n", "")
}
