package integration

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	"github.com/siutsin/telegram-jung2-bot/internal/app"
	appmock "github.com/siutsin/telegram-jung2-bot/internal/mock"
)

// TestFlociAppRunGracefulShutdownAfterComponentCrash proves every failed component drains traffic safely.
func TestFlociAppRunGracefulShutdownAfterComponentCrash(t *testing.T) {
	t.Parallel()

	ctx, _ := requireIntegrationRuntime(t)
	for _, testCase := range []struct {
		name           string
		crash          func(lifecycleApp) error
		wantErr        func(lifecycleApp) error
		drainSupported bool
	}{
		{
			name: "queue worker",
			crash: func(lifecycle lifecycleApp) error {
				lifecycle.crashWorker()
				return nil
			},
			wantErr:        func(lifecycle lifecycleApp) error { return lifecycle.workerErr },
			drainSupported: true,
		},
		{
			name:           "HTTP server",
			crash:          func(lifecycle lifecycleApp) error { return lifecycle.crashHTTP() },
			wantErr:        func(lifecycleApp) error { return http.ErrServerClosed },
			drainSupported: false,
		},
		{
			name:           "metrics server",
			crash:          func(lifecycle lifecycleApp) error { return lifecycle.crashMetrics() },
			wantErr:        func(lifecycleApp) error { return http.ErrServerClosed },
			drainSupported: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			lifecycle := startLifecycleApp(t, ctx)
			client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
			require.Eventually(t, func() bool {
				return lifecycleResponseStatus(client, lifecycle.webhookURL+"/health") == http.StatusOK &&
					lifecycleResponseStatus(client, lifecycle.metricsURL) == http.StatusOK
			}, time.Second, time.Millisecond)
			<-lifecycle.workerStarted

			var drainDone <-chan error
			if testCase.drainSupported {
				drainDone = startDrainRequest(client, lifecycle.webhookURL+"/drain")
				<-lifecycle.drainStarted
			}
			require.NoError(t, testCase.crash(lifecycle))

			require.Eventually(t, func() bool { return !lifecycle.readiness.Load() }, time.Second, time.Millisecond)
			if testCase.drainSupported {
				select {
				case runErr := <-lifecycle.runDone:
					t.Fatalf("app stopped before the in-flight request drained: %v", runErr)
				default:
				}
				lifecycle.releaseDrain()
				require.NoError(t, <-drainDone)
			}
			require.ErrorIs(t, <-lifecycle.runDone, testCase.wantErr(lifecycle))
			require.Eventually(t, func() bool {
				return lifecycleCannotConnect(lifecycle.webhookAddress) &&
					lifecycleCannotConnect(lifecycle.metricsAddress)
			}, time.Second, time.Millisecond)
		})
	}
}

type lifecycleApp struct {
	readiness      *atomic.Bool
	workerStarted  <-chan struct{}
	drainStarted   <-chan struct{}
	workerErr      error
	runDone        <-chan error
	webhookAddress string
	metricsAddress string
	webhookURL     string
	metricsURL     string
	crashWorker    func()
	crashHTTP      func() error
	crashMetrics   func() error
	releaseDrain   func()
}

// startLifecycleApp uses real listeners so lifecycle failures exercise HTTP shutdown behaviour.
func startLifecycleApp(t *testing.T, ctx context.Context) lifecycleApp {
	t.Helper()

	readiness := &atomic.Bool{}
	drainStarted := make(chan struct{})
	releaseDrain := make(chan struct{})
	workerStarted := make(chan struct{})
	crashWorker := make(chan struct{})
	workerErr := errors.New("queue worker crashed")
	var releaseDrainOnce sync.Once
	var crashWorkerOnce sync.Once
	webhookAddress := freeTCPAddress(t)
	metricsAddress := freeTCPAddress(t)
	queueWorker := appmock.NewMockQueueWorker(gomock.NewController(t))
	queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(runCtx context.Context) error {
		close(workerStarted)
		select {
		case <-crashWorker:
			return workerErr
		case <-runCtx.Done():
			return runCtx.Err()
		}
	})
	webhookServer := lifecycleWebhookServer(readiness, drainStarted, releaseDrain)
	metricsServer := lifecycleMetricsServer()
	application := app.New(
		app.NewHTTPServer("HTTP", webhookAddress, webhookServer),
		app.NewHTTPServer("metrics", metricsAddress, metricsServer),
		queueWorker,
		app.Options{Readiness: readiness, ReadinessDrain: time.Nanosecond, ShutdownTimeout: time.Second},
	)
	runDone := make(chan error, 1)
	go func() { runDone <- application.Run(ctx) }()
	t.Cleanup(func() {
		crashWorkerOnce.Do(func() { close(crashWorker) })
		releaseDrainOnce.Do(func() { close(releaseDrain) })
		select {
		case <-runDone:
		case <-time.After(time.Second):
		}
	})

	return lifecycleApp{
		readiness:      readiness,
		workerStarted:  workerStarted,
		drainStarted:   drainStarted,
		workerErr:      workerErr,
		runDone:        runDone,
		webhookAddress: webhookAddress,
		metricsAddress: metricsAddress,
		webhookURL:     "http://" + webhookAddress,
		metricsURL:     "http://" + metricsAddress,
		crashWorker:    func() { crashWorkerOnce.Do(func() { close(crashWorker) }) },
		crashHTTP:      webhookServer.Close,
		crashMetrics:   metricsServer.Close,
		releaseDrain:   func() { releaseDrainOnce.Do(func() { close(releaseDrain) }) },
	}
}

// lifecycleWebhookServer supplies controllable readiness and in-flight work for shutdown assertions.
func lifecycleWebhookServer(readiness *atomic.Bool, drainStarted chan<- struct{}, releaseDrain <-chan struct{}) *http.Server {
	return &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/health":
				if !readiness.Load() {
					writer.WriteHeader(http.StatusServiceUnavailable)
				}
			case "/drain":
				drainStarted <- struct{}{}
				<-releaseDrain
			default:
				writer.WriteHeader(http.StatusNotFound)
			}
		}),
	}
}

// lifecycleMetricsServer provides a real independent server that can fail separately from HTTP.
func lifecycleMetricsServer() *http.Server {
	return &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler:           http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	}
}

// startDrainRequest keeps an HTTP request active until the test permits graceful shutdown.
func startDrainRequest(client *http.Client, url string) <-chan error {
	done := make(chan error, 1)
	go func() {
		response, err := client.Get(url)
		if err != nil {
			done <- err
			return
		}
		if response.StatusCode != http.StatusOK {
			closeErr := response.Body.Close()
			if closeErr != nil {
				done <- fmt.Errorf("close drain response body: %w", closeErr)
				return
			}
			done <- fmt.Errorf("drain response status = %d", response.StatusCode)
			return
		}
		closeErr := response.Body.Close()
		if closeErr != nil {
			done <- fmt.Errorf("close drain response body: %w", closeErr)
			return
		}
		done <- nil
	}()

	return done
}

// lifecycleResponseStatus makes readiness probes retryable without leaking response bodies.
func lifecycleResponseStatus(client *http.Client, url string) int {
	response, err := client.Get(url)
	if err != nil {
		return 0
	}
	status := response.StatusCode
	closeErr := response.Body.Close()
	if closeErr != nil {
		return 0
	}

	return status
}

// lifecycleCannotConnect confirms shutdown closes listeners instead of merely marking them unready.
func lifecycleCannotConnect(address string) bool {
	connection, err := net.DialTimeout("tcp", address, time.Millisecond)
	if err != nil {
		return true
	}

	return connection.Close() == nil
}
