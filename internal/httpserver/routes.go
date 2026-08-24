package httpserver

import (
	"crypto/subtle"
	"io"
	"log/slog"
	"net/http"
	"strings"

	bot "github.com/siutsin/telegram-jung2-bot/internal"
)

// newHandler builds the HTTP handler for service routes.
func newHandler(dependencies serverDeps) http.Handler {
	mux := http.NewServeMux()
	registerRoute(mux, http.MethodGet, "/health", "health", dependencies.Metrics, func(writer http.ResponseWriter, request *http.Request) {
		writeResponse(writer, health(dependencies.Readiness))
	})
	registerRoute(mux, http.MethodPost, "/webhook", "webhook", dependencies.Metrics, func(writer http.ResponseWriter, request *http.Request) {
		writeResponse(writer, webhookResponse(writer, request, dependencies))
	})
	if dependencies.stage != "" {
		registerStageRoutes(mux, dependencies)
	}
	mux.Handle("/", instrumentRoute(dependencies.Metrics, "not_found", http.NotFoundHandler()))

	return mux
}

// registerStageRoutes wires the contract-compatible stage-prefixed routes.
func registerStageRoutes(mux *http.ServeMux, dependencies serverDeps) {
	stagePrefix := "/jung2bot/" + strings.Trim(dependencies.stage, "/")
	registerRoute(mux, http.MethodGet, stagePrefix+"/ping", "stage_ping", dependencies.Metrics, func(writer http.ResponseWriter, request *http.Request) {
		writeNamedJSONResponse(writer, http.StatusOK, "health", "ok")
	})

	mux.Handle(stagePrefix, instrumentRoute(dependencies.Metrics, "not_found", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	})))

	registerRoute(mux, http.MethodPost, stagePrefix+"/", "stage_webhook", dependencies.Metrics, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != stagePrefix+"/" {
			http.NotFound(writer, request)
			return
		}
		writeStageWebhookResponse(writer, webhookResponse(writer, request, dependencies))
	})

	registerRoute(mux, http.MethodGet, stagePrefix+"/onOffFromWork", "off_work_scheduler", dependencies.Metrics, func(writer http.ResponseWriter, request *http.Request) {
		if rejectUnauthorisedStageRoute(writer, request, dependencies.SchedulerSecretToken, "onOffFromWork") {
			return
		}

		_, err := bot.ParseScheduledTime(request.URL.Query().Get("timeString"))
		if err != nil {
			writeNamedJSONResponse(writer, http.StatusBadRequest, "onOffFromWork", "invalid timeString")
			return
		}

		err = dependencies.Enqueuer.Enqueue(request.Context(), bot.BuildOnOffFromWorkAction(request.URL.Query().Get("timeString")))
		if err != nil {
			slog.Error("enqueue off-work trigger", "err", err)
			writeNamedJSONResponse(writer, http.StatusInternalServerError, "onOffFromWork", "failed")
			return
		}
		writeNamedJSONResponse(writer, http.StatusAccepted, "onOffFromWork", "ok")
	})

	registerRoute(mux, http.MethodGet, stagePrefix+"/onScaleUp", "scale_up", dependencies.Metrics, func(writer http.ResponseWriter, request *http.Request) {
		if rejectUnauthorisedStageRoute(writer, request, dependencies.SchedulerSecretToken, "onScaleUp") {
			return
		}

		if dependencies.ScaleUpper == nil {
			slog.Error("scale up dependency missing")
			writeNamedJSONResponse(writer, http.StatusServiceUnavailable, "onScaleUp", "failed")
			return
		}
		err := dependencies.ScaleUpper.ScaleUp(request.Context())
		if err != nil {
			slog.Error("scale up", "err", err)
			writeNamedJSONResponse(writer, http.StatusServiceUnavailable, "onScaleUp", "failed")
			return
		}
		writeNamedJSONResponse(writer, http.StatusOK, "onScaleUp", "ok")
	})
}

// registerRoute wires one named route with its required HTTP method.
func registerRoute(mux *http.ServeMux, method string, path string, route string, metrics metricsRecorder, handler http.HandlerFunc) {
	mux.Handle(path, instrumentRoute(metrics, route, methodHandler(method, handler)))
}

// instrumentRoute adds route metrics when the application supplies a recorder.
func instrumentRoute(metrics metricsRecorder, route string, handler http.Handler) http.Handler {
	if metrics == nil {
		return handler
	}

	return metrics.HTTPHandler(route, handler)
}

// methodHandler rejects requests that do not match the configured route method.
func methodHandler(method string, handler http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != method {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(writer, request)
	}
}

// webhookResponse reads the HTTP request body and processes one webhook update.
func webhookResponse(writer http.ResponseWriter, request *http.Request, dependencies serverDeps) response {
	if !validateWebhookSecret(request, dependencies.WebhookSecretToken) {
		result := response{statusCode: http.StatusUnauthorized, message: "unauthorised"}
		recordWebhookOutcome(dependencies.Metrics, result.statusCode)
		return result
	}

	body, err := readRequestBody(writer, request, maxBodyBytes(dependencies))
	if err != nil {
		slog.Warn("read webhook request body", "err", err)
		result := response{statusCode: http.StatusBadRequest, message: "read request body"}
		recordWebhookOutcome(dependencies.Metrics, result.statusCode)
		return result
	}

	result := handleWebhook(request.Context(), body, dependencies.Dependencies)
	recordWebhookOutcome(dependencies.Metrics, result.statusCode)
	return result
}

// recordWebhookOutcome maps HTTP responses to the fixed webhook outcome labels.
func recordWebhookOutcome(metrics metricsRecorder, statusCode int) {
	if metrics == nil {
		return
	}

	switch statusCode {
	case http.StatusOK:
		metrics.RecordWebhookUpdate("processed")
	case http.StatusNoContent:
		metrics.RecordWebhookUpdate("ignored")
	case http.StatusUnauthorized, http.StatusBadRequest:
		metrics.RecordWebhookUpdate("rejected")
	default:
		metrics.RecordWebhookUpdate("failed")
	}
}

// readRequestBody reads a bounded request body.
func readRequestBody(writer http.ResponseWriter, request *http.Request, bodyLimit int64) ([]byte, error) {
	body := http.MaxBytesReader(writer, request.Body, bodyLimit)
	defer func() {
		err := body.Close()
		if err != nil {
			logHTTPError("close request body", http.StatusBadRequest, err)
		}
	}()

	return io.ReadAll(body)
}

// rejectUnauthorisedStageRoute writes 401 when the configured scheduler secret
// is missing or wrong. For example, a bad onScaleUp request becomes
// {"onScaleUp":"unauthorised"}.
func rejectUnauthorisedStageRoute(writer http.ResponseWriter, request *http.Request, secret string, routeName string) bool {
	if validateSchedulerSecret(request, secret) {
		return false
	}

	writeNamedJSONResponse(writer, http.StatusUnauthorized, routeName, "unauthorised")
	return true
}

// validateSchedulerSecret checks scheduler auth via query or bearer token.
// For example, ?schedulerToken=secret matches SCHEDULER_SECRET_TOKEN.
func validateSchedulerSecret(request *http.Request, secret string) bool {
	if secret == "" {
		return true
	}

	queryToken := request.URL.Query().Get("schedulerToken")
	if subtle.ConstantTimeCompare([]byte(queryToken), []byte(secret)) == 1 {
		return true
	}

	authHeader := request.Header.Get("Authorization")
	if len(authHeader) > len("Bearer ") && authHeader[:len("Bearer ")] == "Bearer " {
		bearer := authHeader[len("Bearer "):]
		return subtle.ConstantTimeCompare([]byte(bearer), []byte(secret)) == 1
	}

	return false
}

// validateWebhookSecret checks Telegram's webhook secret header when configured.
func validateWebhookSecret(request *http.Request, secret string) bool {
	if secret == "" {
		return true
	}

	got := request.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	return subtle.ConstantTimeCompare([]byte(got), []byte(secret)) == 1
}
