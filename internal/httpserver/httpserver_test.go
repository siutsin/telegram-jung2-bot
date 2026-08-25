package httpserver

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	readiness := &atomic.Bool{}
	assert.Equal(t, response{statusCode: http.StatusServiceUnavailable, message: "not ready"}, health(readiness))

	readiness.Store(true)
	assert.Equal(t, response{statusCode: http.StatusOK, message: "ok"}, health(readiness))
}

func TestHealthRouteReadsReadiness(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	handler := newHandler(serverDeps{Dependencies: dependencies})
	dependencies.Readiness.Store(false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t, "not ready", recorder.Body.String())

	dependencies.Readiness.Store(true)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
}

func TestNewServerBuildsConfiguredHTTPServer(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	server, err := NewServer(
		":3000",
		5*time.Second,
		"dev",
		dependencies,
	)

	require.NoError(t, err)
	assert.Equal(t, ":3000", server.Addr)
	assert.Equal(t, 5*time.Second, server.ReadTimeout)
	assert.Equal(t, 5*time.Second, server.WriteTimeout)

	recorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/ping", nil))
	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestNewServerRejectsInvalidDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewServer(":3000", 5*time.Second, "dev", Dependencies{})

	require.Error(t, err)
	assert.EqualError(t, err, "validate HTTP dependencies: enqueuer is required")
}
