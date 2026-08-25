package integration

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	bot "github.com/siutsin/telegram-jung2-bot/internal"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

func runAppRunIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	messenger := &recordingMessenger{}
	svc := newIntegrationService(dynamoClient, sqsClient, resources, messenger)
	queueClient := queue.NewClient(sqsClient)

	queueWorker, err := bot.NewPollingWorker(
		resources.queueURL,
		queueClient,
		queueClient,
		buildWorkerHandlers(svc),
	)
	require.NoError(t, err, "create app queue worker")

	queueProducer := queue.NewProducer(resources.queueURL, queueClient)
	readiness := &atomic.Bool{}
	deps := httpserver.Dependencies{
		ChatTable:       resources.chatTable,
		MessageTable:    resources.messageTable,
		Enqueuer:        queueProducer,
		MessageEnqueuer: queueProducer,
		Messenger:       noopMessenger{},
		Now: func() time.Time {
			return integrationNow
		},
		Readiness: readiness,
	}
	httpAddress := freeTCPAddress(t)
	httpServer, err := httpserver.NewServer(httpAddress, 5*time.Second, "", deps)
	require.NoError(t, err, "create app HTTP server")
	metricsAddress := freeTCPAddress(t)
	metricsServer := &http.Server{
		Addr:              metricsAddress,
		Handler:           http.NotFoundHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       5 * time.Second,
	}
	application := bot.NewApp(
		bot.NewHTTPServer("HTTP", httpAddress, httpServer),
		bot.NewHTTPServer("metrics", metricsAddress, metricsServer),
		queueWorker,
		bot.AppOptions{Readiness: readiness, ReadinessDrain: time.Nanosecond, ShutdownTimeout: 5 * time.Second},
	)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- application.Run(runCtx)
	}()

	healthURL := "http://" + httpAddress + "/health"
	require.Eventually(t, func() bool {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if requestErr != nil {
			return false
		}

		response, healthErr := http.DefaultClient.Do(request)
		if healthErr != nil {
			return false
		}
		defer func() {
			closeErr := response.Body.Close()
			if closeErr != nil {
				t.Errorf("close health response body: %v", closeErr)
			}
		}()

		if response.StatusCode != http.StatusOK {
			return false
		}
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return false
		}

		return string(payload) == "ok"
	}, 15*time.Second, 100*time.Millisecond, "app health endpoint should become ready")

	const (
		appChatID    int64 = 42021
		appChatTitle       = "App Run Integration"
		appUserID    int64 = 10021
	)
	action := mustCommandAction(t, "/jungHelp", appChatID, appChatTitle, appUserID)
	err = queueProducer.Enqueue(ctx, action)
	require.NoError(t, err, "enqueue app run action")

	require.Eventually(t, func() bool {
		return len(messenger.recordedMessages()) > 0
	}, 15*time.Second, 100*time.Millisecond, "app run should process queued action")

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, appChatID, messages[0].chatID)

	cancel()

	select {
	case runErr := <-done:
		require.NoError(t, runErr, "app run should shut down cleanly")
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for app run to stop")
	}

	assertQueueEmpty(t, ctx, queueClient, resources.queueURL)
}

func freeTCPAddress(t *testing.T) string {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err, "reserve TCP address")
	address := listener.Addr().String()
	require.NoError(t, listener.Close(), "release TCP address")

	return address
}
