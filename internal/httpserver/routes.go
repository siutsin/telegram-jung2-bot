package httpserver

import (
	"crypto/subtle"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
)

// newHandler builds the HTTP handler for service routes.
func newHandler(dependencies serverDeps) http.Handler {
	mux := http.NewServeMux()
	registerRoute(mux, http.MethodGet, "/health", func(writer http.ResponseWriter, request *http.Request) {
		writeResponse(writer, health())
	})
	registerRoute(mux, http.MethodPost, "/webhook", func(writer http.ResponseWriter, request *http.Request) {
		writeResponse(writer, webhookResponse(writer, request, dependencies))
	})
	if dependencies.stage != "" {
		registerStageRoutes(mux, dependencies)
	}

	return mux
}

// registerStageRoutes wires the contract-compatible stage-prefixed routes.
func registerStageRoutes(mux *http.ServeMux, dependencies serverDeps) {
	stagePrefix := "/jung2bot/" + strings.Trim(dependencies.stage, "/")
	registerRoute(mux, http.MethodGet, stagePrefix+"/ping", func(writer http.ResponseWriter, request *http.Request) {
		writeNamedJSONResponse(writer, http.StatusOK, "health", "ok")
	})

	mux.HandleFunc(stagePrefix, func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	})

	registerRoute(mux, http.MethodPost, stagePrefix+"/", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != stagePrefix+"/" {
			http.NotFound(writer, request)
			return
		}
		writeStageWebhookResponse(writer, webhookResponse(writer, request, dependencies))
	})

	registerRoute(mux, http.MethodGet, stagePrefix+"/onOffFromWork", func(writer http.ResponseWriter, request *http.Request) {
		if rejectUnauthorisedStageRoute(writer, request, dependencies.SchedulerSecretToken, "onOffFromWork") {
			return
		}

		_, err := schedule.ParseScheduledTime(request.URL.Query().Get("timeString"))
		if err != nil {
			writeNamedJSONResponse(writer, http.StatusBadRequest, "onOffFromWork", "invalid timeString")
			return
		}

		err = dependencies.Enqueuer.Enqueue(request.Context(), schedule.BuildOnOffFromWorkAction(request.URL.Query().Get("timeString")))
		if err != nil {
			slog.Error("enqueue off-work trigger", "err", err)
			writeNamedJSONResponse(writer, http.StatusInternalServerError, "onOffFromWork", "failed")
			return
		}
		writeNamedJSONResponse(writer, http.StatusAccepted, "onOffFromWork", "ok")
	})

	registerRoute(mux, http.MethodGet, stagePrefix+"/onScaleUp", func(writer http.ResponseWriter, request *http.Request) {
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

// registerRoute wires one route with its required HTTP method.
func registerRoute(mux *http.ServeMux, method string, path string, handler http.HandlerFunc) {
	mux.HandleFunc(path, methodHandler(method, handler))
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
		return response{statusCode: http.StatusUnauthorized, message: "unauthorised"}
	}

	body, err := readRequestBody(writer, request, maxBodyBytes(dependencies))
	if err != nil {
		slog.Warn("read webhook request body", "err", err)
		return response{statusCode: http.StatusBadRequest, message: "read request body"}
	}

	return handleWebhook(request.Context(), body, dependencies.Dependencies)
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
