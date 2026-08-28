package bot

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsHandlerExposesServiceSignals proves a scrape includes intended
// runtime and service signals without leaking the global registry.
func TestMetricsHandlerExposesServiceSignals(t *testing.T) {
	t.Parallel()

	globalMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_global_prometheus_metric",
		Help: "Metric registered only on the global Prometheus registry.",
	})
	require.NoError(t, prometheus.Register(globalMetric))
	t.Cleanup(func() { prometheus.Unregister(globalMetric) })
	globalMetric.Set(1)

	readiness := &atomic.Bool{}
	metrics := NewMetrics(readiness)
	metrics.HTTPHandler("webhook", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusAccepted)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/webhook", nil))

	body := scrapeMetrics(t, metrics)
	assert.Contains(t, body, "telegram_jung2_bot_ready 0")
	assert.Contains(t, body, "telegram_jung2_bot_http_requests_total{code=\"202\",method=\"post\",route=\"webhook\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_http_request_duration_seconds_count{code=\"202\",method=\"post\",route=\"webhook\"} 1")
	assert.Contains(t, body, "go_gc_duration_seconds", "scrapes include Go runtime metrics")
	assert.Contains(t, body, "process_start_time_seconds", "scrapes include process metrics")
	assert.NotContains(t, body, "test_global_prometheus_metric", "scrapes must not include global Prometheus metrics")

	readiness.Store(true)
	assert.Contains(t, scrapeMetrics(t, metrics), "telegram_jung2_bot_ready 1")
}

// TestMetricsRecordsApplicationSignals proves all application metric groups are scrapeable.
func TestMetricsRecordsApplicationSignals(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(&atomic.Bool{})
	metrics.HTTPHandler("health", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, err := writer.Write([]byte("ok"))
		assert.NoError(t, err)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	metrics.RecordWebhookUpdate("processed")
	metrics.RecordWebhookUpdate("ignored")
	metrics.RecordWebhookUpdate("rejected")
	metrics.RecordWebhookUpdate("failed")
	metrics.RecordWebhookCommand("topten")
	metrics.RecordWorkerAction("topten", "processed", time.Millisecond)
	metrics.RecordWorkerAction("unknown", "discarded", time.Millisecond)
	metrics.RecordWorkerAction("topten", "failed", time.Millisecond)
	metrics.RecordWorkerAction("topten", "dropped", time.Millisecond)
	metrics.RecordQueueWait("topten", time.Second)
	metrics.ObserveDependency("dynamodb", "query", time.Millisecond, nil)
	metrics.ObserveDependency("telegram", "send_message", time.Millisecond, errors.New("unavailable"))
	metrics.RecordOffWorkReportEnqueue(nil)
	metrics.RecordOffWorkReportEnqueue(errors.New("unavailable"))
	metrics.RecordScaleUp("applied")
	metrics.RecordScaleUp("ignored")
	metrics.RecordScaleUp("failed")

	body := scrapeMetrics(t, metrics)
	assert.Contains(t, body, "telegram_jung2_bot_http_requests_total{code=\"200\",method=\"get\",route=\"health\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_webhook_updates_total{outcome=\"processed\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_webhook_commands_enqueued_total{action=\"topten\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_worker_actions_total{action=\"topten\",outcome=\"processed\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_worker_actions_total{action=\"topten\",outcome=\"dropped\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_worker_queue_wait_duration_seconds_count{action=\"topten\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_dependency_requests_total{dependency=\"dynamodb\",operation=\"query\",result=\"success\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_dependency_requests_total{dependency=\"telegram\",operation=\"send_message\",result=\"error\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_off_work_reports_enqueued_total{result=\"success\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_off_work_reports_enqueued_total{result=\"error\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_scale_up_total{result=\"applied\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_scale_up_total{result=\"ignored\"} 1")
	assert.Contains(t, body, "telegram_jung2_bot_scale_up_total{result=\"failed\"} 1")
}

// TestMetricsResponseWriterRetainsTheFirstStatus proves route metrics use the sent response status.
func TestMetricsResponseWriterRetainsTheFirstStatus(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	writer := metricsResponseWriter{ResponseWriter: recorder, statusCode: http.StatusOK}
	writer.WriteHeader(http.StatusAccepted)
	writer.WriteHeader(http.StatusInternalServerError)

	assert.Equal(t, http.StatusAccepted, writer.statusCode)
	assert.Same(t, recorder, writer.Unwrap())
}

// scrapeMetrics reads one scrape response and fails the test on transport errors.
func scrapeMetrics(t *testing.T, metrics *Metrics) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	response := recorder.Result()
	t.Cleanup(func() { require.NoError(t, response.Body.Close()) })
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	return string(body)
}
