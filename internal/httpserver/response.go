package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeResponse writes a plain response body.
func writeResponse(writer http.ResponseWriter, result response) {
	writer.WriteHeader(result.StatusCode)
	if result.Message != "" && allowsResponseBody(result.StatusCode) {
		_, err := writer.Write([]byte(result.Message))
		if err != nil {
			logHTTPError("write plain response", result.StatusCode, err)
		}
	}
}

// writeStageWebhookResponse writes the stage-compatible webhook response.
// For example, response{StatusCode: 200, Message: "ok"} becomes
// {"statusCode":200,"message":"ok"}.
func writeStageWebhookResponse(writer http.ResponseWriter, result response) {
	body := map[string]any{"statusCode": result.StatusCode}
	if result.Message != "" && result.StatusCode < http.StatusInternalServerError {
		body["message"] = result.Message
	}
	writeJSONResponse(writer, result.StatusCode, body)
}

// writeNamedJSONResponse writes the legacy static route response shape.
func writeNamedJSONResponse(writer http.ResponseWriter, statusCode int, name string, value string) {
	writeJSONResponse(writer, statusCode, map[string]string{name: value})
}

// writeJSONResponse writes a JSON response body.
// For example, body map[string]string{"ping":"pong"} becomes a JSON object
// response.
func writeJSONResponse(writer http.ResponseWriter, statusCode int, body any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	err := json.NewEncoder(writer).Encode(body)
	if err != nil {
		logHTTPError("encode JSON response", statusCode, err)
	}
}

// allowsResponseBody reports whether an HTTP status permits a response body.
// For example, 204 returns false, while 200 returns true.
func allowsResponseBody(statusCode int) bool {
	return statusCode != http.StatusNoContent && statusCode != http.StatusNotModified
}

// logHTTPError records HTTP errors after headers or request state are in flight.
func logHTTPError(operation string, statusCode int, err error) {
	slog.Error(operation, "status_code", statusCode, "err", err)
}
