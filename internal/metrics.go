package bot

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsNamespace = "telegram_jung2_bot"

// Metrics collects low-cardinality signals for the running service.
type Metrics struct {
	registry                *prometheus.Registry
	inFlight                prometheus.Gauge
	requestCount            *prometheus.CounterVec
	requestTiming           *prometheus.HistogramVec
	webhookUpdates          *prometheus.CounterVec
	webhookCommands         *prometheus.CounterVec
	workerActions           *prometheus.CounterVec
	workerActionTiming      *prometheus.HistogramVec
	dependencyRequests      *prometheus.CounterVec
	dependencyRequestTiming *prometheus.HistogramVec
	offWorkReportsEnqueued  *prometheus.CounterVec
}

// NewMetrics creates an isolated registry for this process.
// For example, a false readiness value exposes telegram_jung2_bot_ready 0.
func NewMetrics(readiness *atomic.Bool) *Metrics {
	metrics := &Metrics{registry: prometheus.NewRegistry()}
	metrics.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	factory := promauto.With(metrics.registry)

	metrics.registerHTTP(factory)
	metrics.registerWebhook(factory)
	metrics.registerWorker(factory)
	metrics.registerDependencies(factory)
	metrics.registerScheduler(factory)
	metrics.registerReadiness(factory, readiness)

	return metrics
}

// registerHTTP creates the metrics that describe requests to fixed public routes.
func (metrics *Metrics) registerHTTP(factory promauto.Factory) {
	metrics.inFlight = factory.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "http_requests_in_flight",
		Help:      "Current number of HTTP requests being handled.",
	})
	metrics.requestCount = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "http_requests_total",
		Help:      "Total HTTP requests handled.",
	}, []string{"code", "method", "route"})
	metrics.requestTiming = factory.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request duration in seconds.",
	}, []string{"code", "method", "route"})
}

// registerWebhook creates the metrics that describe accepted Telegram updates.
func (metrics *Metrics) registerWebhook(factory promauto.Factory) {
	metrics.webhookUpdates = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "webhook_updates_total",
		Help:      "Total Telegram webhook updates by outcome.",
	}, []string{"outcome"})
	metrics.webhookCommands = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "webhook_commands_enqueued_total",
		Help:      "Total webhook commands successfully queued.",
	}, []string{"action"})
}

// registerWorker creates the metrics that describe queue action processing.
func (metrics *Metrics) registerWorker(factory promauto.Factory) {
	metrics.workerActions = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "worker_actions_total",
		Help:      "Total queue actions by outcome.",
	}, []string{"action", "outcome"})
	metrics.workerActionTiming = factory.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Name:      "worker_action_duration_seconds",
		Help:      "Queue action processing duration in seconds.",
	}, []string{"action"})
}

// registerDependencies creates the metrics that describe outbound service calls.
func (metrics *Metrics) registerDependencies(factory promauto.Factory) {
	metrics.dependencyRequests = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "dependency_requests_total",
		Help:      "Total outbound dependency requests by result.",
	}, []string{"dependency", "operation", "result"})
	metrics.dependencyRequestTiming = factory.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Name:      "dependency_request_duration_seconds",
		Help:      "Outbound dependency request duration in seconds.",
	}, []string{"dependency", "operation"})
}

// registerScheduler creates the metric for scheduled report enqueue attempts.
func (metrics *Metrics) registerScheduler(factory promauto.Factory) {
	metrics.offWorkReportsEnqueued = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "off_work_reports_enqueued_total",
		Help:      "Total scheduled off-work reports queued by result.",
	}, []string{"result"})
}

// registerReadiness creates the gauge that Kubernetes uses to decide when to send traffic.
func (metrics *Metrics) registerReadiness(factory promauto.Factory, readiness *atomic.Bool) {
	factory.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "ready",
		Help:      "Whether the service is ready for new traffic.",
	}, func() float64 {
		if readiness != nil && readiness.Load() {
			return 1
		}

		return 0
	})
}

// HTTPHandler instruments one fixed public HTTP route.
// For example, a POST to the webhook route that returns 202 records post, 202,
// and webhook without adding request paths as labels.
func (metrics *Metrics) HTTPHandler(route string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		metrics.inFlight.Inc()
		defer metrics.inFlight.Dec()

		started := time.Now()
		response := metricsResponseWriter{ResponseWriter: writer, statusCode: http.StatusOK}
		handler.ServeHTTP(&response, request)
		labels := prometheus.Labels{
			"code":   strconv.Itoa(response.statusCode),
			"method": strings.ToLower(request.Method),
			"route":  route,
		}
		metrics.requestCount.With(labels).Inc()
		metrics.requestTiming.With(labels).Observe(time.Since(started).Seconds())
	})
}

// RecordWebhookUpdate records the final result of one Telegram update.
func (metrics *Metrics) RecordWebhookUpdate(outcome string) {
	metrics.webhookUpdates.WithLabelValues(outcome).Inc()
}

// RecordWebhookCommand records one successfully queued command.
func (metrics *Metrics) RecordWebhookCommand(action string) {
	metrics.webhookCommands.WithLabelValues(action).Inc()
}

// RecordWorkerAction records the result and duration of one queue action.
func (metrics *Metrics) RecordWorkerAction(action string, outcome string, duration time.Duration) {
	metrics.workerActions.WithLabelValues(action, outcome).Inc()
	metrics.workerActionTiming.WithLabelValues(action).Observe(duration.Seconds())
}

// ObserveDependency records one outbound call to a fixed dependency operation.
func (metrics *Metrics) ObserveDependency(dependency string, operation string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	metrics.dependencyRequests.WithLabelValues(dependency, operation, result).Inc()
	metrics.dependencyRequestTiming.WithLabelValues(dependency, operation).Observe(duration.Seconds())
}

// RecordOffWorkReportEnqueue records one attempt to queue a scheduled report.
func (metrics *Metrics) RecordOffWorkReportEnqueue(err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	metrics.offWorkReportsEnqueued.WithLabelValues(result).Inc()
}

// metricsResponseWriter records the first HTTP status sent by an instrumented route.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

// WriteHeader sends a status once and retains it for the route metrics.
func (writer *metricsResponseWriter) WriteHeader(statusCode int) {
	if writer.wroteHeader {
		return
	}
	writer.statusCode = statusCode
	writer.wroteHeader = true
	writer.ResponseWriter.WriteHeader(statusCode)
}

// Write sends a default successful status before response data.
func (writer *metricsResponseWriter) Write(data []byte) (int, error) {
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}

	return writer.ResponseWriter.Write(data)
}

// Unwrap exposes the original writer to HTTP response helpers.
func (writer *metricsResponseWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

// Handler returns the Prometheus scrape handler for this service registry.
func (metrics *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{})
}
