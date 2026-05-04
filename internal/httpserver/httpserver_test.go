package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	assert.Equal(t, response{StatusCode: 200, Message: "ok"}, health())
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
	assert.EqualError(t, err, "validate HTTP dependencies: message store is required")
}
