package httpserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

func TestNewRoutesHealth(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	handler := newHandler(serverDeps{Dependencies: dependencies})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
}

func TestNewRejectsUnsupportedMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		stage  string
	}{
		{
			name:   "health",
			method: http.MethodPost,
			path:   "/health",
		},
		{
			name:   "webhook",
			method: http.MethodGet,
			path:   "/webhook",
		},
		{
			name:   "off from work",
			method: http.MethodPost,
			path:   "/jung2bot/dev/onOffFromWork",
			stage:  "dev",
		},
		{
			name:   "scale up",
			method: http.MethodPost,
			path:   "/jung2bot/dev/onScaleUp",
			stage:  "dev",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, dependencies := newMockDependencies(t)
			handler := newHandler(serverDeps{
				Dependencies: dependencies,
				stage:        tc.stage,
			})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))

			assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
		})
	}
}

func TestNewRoutesWebhook(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	mocks.expectEnqueue(nil)
	handler := newHandler(serverDeps{Dependencies: dependencies})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/topTen","entities":[{"type":"bot_command"}]}}`))

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Len(t, mocks.savedMessages, 1)
	assert.Len(t, mocks.actions, 1)
}

func TestNewRejectsOversizedWebhookBody(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	handler := newHandler(serverDeps{Dependencies: dependencies, maxBodyBytes: 1})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("{}")))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, "read request body", recorder.Body.String())
}

func TestNewUsesDefaultWebhookBodyLimit(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	handler := newHandler(serverDeps{Dependencies: dependencies})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"edited_message":{"text":"ignored"}}`)))

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestNewRoutesContractWebhookAndHealthPaths(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	handler := newHandler(serverDeps{Dependencies: dependencies, stage: "dev"})

	healthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/ping", nil))
	assert.Equal(t, http.StatusOK, healthRecorder.Code)
	assert.JSONEq(t, `{"health":"ok"}`, healthRecorder.Body.String())

	webhook := httptest.NewRecorder()
	handler.ServeHTTP(webhook, httptest.NewRequest(http.MethodPost, "/jung2bot/dev/", strings.NewReader(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"hi"}}`)))
	assert.Equal(t, http.StatusOK, webhook.Code)
	assert.JSONEq(t, `{"statusCode":200}`, webhook.Body.String())
	assert.Len(t, mocks.savedMessages, 1)
}

func TestNewContractWebhookPathMatching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "extra path",
			path: "/jung2bot/dev/extra",
		},
		{
			name: "missing trailing slash",
			path: "/jung2bot/dev",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, dependencies := newMockDependencies(t)
			handler := newHandler(serverDeps{Dependencies: dependencies, stage: "dev"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{}`)))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestNewRoutesContractOffFromWork(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectEnqueue(nil)
	handler := newHandler(serverDeps{Dependencies: dependencies, stage: "dev"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onOffFromWork?timeString=2026-05-02T12:00:00Z", nil))

	assert.Equal(t, http.StatusAccepted, recorder.Code)
	assert.JSONEq(t, `{"onOffFromWork":"ok"}`, recorder.Body.String())
	require.Len(t, mocks.actions, 1)
	assert.Equal(t, queue.ActionOnOffFromWork, mocks.actions[0].Name)
	assert.Equal(t, "2026-05-02T12:00:00Z", mocks.actions[0].Attributes["timeString"])
}

func TestNewContractOffFromWorkReturnsServerError(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectEnqueue(errors.New("boom"))
	handler := newHandler(serverDeps{Dependencies: dependencies, stage: "dev"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onOffFromWork?timeString=2026-05-02T12:00:00Z", nil))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.JSONEq(t, `{"onOffFromWork":"failed"}`, recorder.Body.String())
}

func TestNewRoutesContractScaleUp(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectScaleUp(nil)
	dependencies.ScaleUpper = mocks.scaleUpper
	handler := newHandler(serverDeps{Dependencies: dependencies, stage: "dev"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onScaleUp", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"onScaleUp":"ok"}`, recorder.Body.String())
}

func TestNewContractScaleUpFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		expectScaleUp  bool
		scaleUpperErr  error
		withScaleUpper bool
	}{
		{
			name:           "dependency error",
			expectScaleUp:  true,
			scaleUpperErr:  errors.New("boom"),
			withScaleUpper: true,
		},
		{
			name: "missing dependency",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mocks, dependencies := newMockDependencies(t)
			if tc.expectScaleUp {
				mocks.expectScaleUp(tc.scaleUpperErr)
			}
			if tc.withScaleUpper {
				dependencies.ScaleUpper = mocks.scaleUpper
			}
			handler := newHandler(serverDeps{Dependencies: dependencies, stage: "dev"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onScaleUp", nil))

			assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			assert.JSONEq(t, `{"onScaleUp":"failed"}`, recorder.Body.String())
		})
	}
}

func TestWebhookRejectsMissingSecretWhenConfigured(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	dependencies.WebhookSecretToken = "secret-token"
	handler := newHandler(serverDeps{Dependencies: dependencies})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{}`)))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestOnOffFromWorkRejectsInvalidTimeString(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	handler := newHandler(serverDeps{Dependencies: dependencies, stage: "dev"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onOffFromWork?timeString=bad", nil))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.JSONEq(t, `{"onOffFromWork":"invalid timeString"}`, recorder.Body.String())
}

func TestNewContractWebhookSuppressesInternalErrorMessage(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	handler := newHandler(serverDeps{Dependencies: dependencies, stage: "dev"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/jung2bot/dev/", strings.NewReader(`{bad json`)))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.JSONEq(t, `{"statusCode":500}`, recorder.Body.String())
}
