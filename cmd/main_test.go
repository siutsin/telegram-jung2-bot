package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	bot "github.com/siutsin/telegram-jung2-bot/internal"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

// TestNewMetricsServerUsesAppConfig proves tests can construct the app with selected dependencies.
func TestNewMetricsServerUsesAppConfig(t *testing.T) {
	testApp := app{
		config:  bot.Config{MetricsServerAddress: "127.0.0.1:9090", HTTPTimeout: time.Second},
		metrics: bot.NewMetrics(nil),
	}

	server := newMetricsServer(testApp)

	assert.Equal(t, testApp.config.MetricsServerAddress, server.Addr)
	assert.Equal(t, time.Second, server.ReadTimeout)
	assert.Equal(t, time.Second, server.ReadHeaderTimeout)
	assert.Equal(t, time.Second, server.WriteTimeout)
	assert.Equal(t, time.Second, server.IdleTimeout)
	require.NotNil(t, server.Handler)
}

// TestNewHTTPServerUsesAppMessageEnqueuer proves a test can replace one app dependency.
func TestNewHTTPServerUsesAppMessageEnqueuer(t *testing.T) {
	controller := gomock.NewController(t)
	messageQueue := NewMockMessageEnqueuer(controller)
	var action queue.Action
	messageQueue.EXPECT().Enqueue(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, got queue.Action) error {
		action = got
		return nil
	})
	testApp := app{
		config: bot.Config{
			ChatIDTable:   "chat-table",
			MessageTable:  "message-table",
			EventQueueURL: "https://sqs.example.com/events",
		},
		readiness:       &atomic.Bool{},
		metrics:         bot.NewMetrics(nil),
		messenger:       bot.NewClient("token"),
		messageEnqueuer: messageQueue,
	}

	server, err := newHTTPServer(testApp)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id":7,"message":{"message_id":8,"date":1777723200,"chat":{"id":9,"type":"group"}}}`))
	server.Handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, queue.ActionSaveMessage, action.Name)
}
