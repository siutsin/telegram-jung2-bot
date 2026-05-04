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

	writeResponse(writer, response{StatusCode: http.StatusOK, Message: "ok"})
}

func TestWriteStageWebhookResponseIncludesSuccessMessage(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()

	writeStageWebhookResponse(recorder, response{StatusCode: http.StatusAccepted, Message: "ok"})

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
