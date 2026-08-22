package httpserver

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorResponseWriter struct {
	header http.Header
}

func (writer *errorResponseWriter) Header() http.Header {
	return writer.header
}

func (writer *errorResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (writer *errorResponseWriter) WriteHeader(int) {}

type closeErrorBody struct {
	reader io.Reader
}

func (body closeErrorBody) Read(bytes []byte) (int, error) {
	return body.reader.Read(bytes)
}

func (body closeErrorBody) Close() error {
	return errors.New("close failed")
}

func TestWriteResponseLogsWriteError(t *testing.T) {
	t.Parallel()

	writer := &errorResponseWriter{header: make(http.Header)}

	writeResponse(writer, response{statusCode: http.StatusOK, message: "ok"})
}

func TestWriteStageWebhookResponseIncludesSuccessMessage(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()

	writeStageWebhookResponse(recorder, response{statusCode: http.StatusAccepted, message: "ok"})

	assert.Equal(t, http.StatusAccepted, recorder.Code)
	assert.JSONEq(t, `{"statusCode":202,"message":"ok"}`, recorder.Body.String())
}

func TestWriteJSONResponseLogsEncodeError(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()

	writeJSONResponse(recorder, http.StatusOK, make(chan int))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
}

// TestWriteJSONResponseSkipsBodyForNoContent covers a 204 stage-webhook
// response (an edited_message or non-group-chat update). Encoding a body
// after a 204 header trips Go's net/http body-not-allowed check, so
// writeJSONResponse must not attempt it.
func TestWriteJSONResponseSkipsBodyForNoContent(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()

	writeJSONResponse(recorder, http.StatusNoContent, map[string]string{"statusCode": "204"})

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Empty(t, recorder.Body.String())
}

func TestReadRequestBodyLogsCloseError(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/webhook", closeErrorBody{
		reader: strings.NewReader("ok"),
	})

	body, readErr := readRequestBody(recorder, request, 10)

	require.NoError(t, readErr)
	assert.Equal(t, []byte("ok"), body)
}
