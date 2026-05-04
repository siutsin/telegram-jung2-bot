package app

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	gomock "go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appmock "github.com/siutsin/telegram-jung2-bot/internal/mock"
)

func TestRunStartsProcessesAndCancelsWorker(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	shutdown := make(chan struct{})
	var shutdownCalled atomic.Bool
	var workerCancelled atomic.Bool
	httpServer, queueWorker := newMocks(t)
	httpServer.EXPECT().ListenAndServe().DoAndReturn(func() error {
		close(started)
		<-shutdown
		return nil
	})
	httpServer.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		shutdownCalled.Store(true)
		close(shutdown)
		return nil
	})
	queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		<-mockCtx.Done()
		workerCancelled.Store(true)
		return nil
	})
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Second})

	require.NoError(t, runWithCancel(t, application, ctx, cancel, started))
	require.Eventually(t, shutdownCalled.Load, time.Second, time.Millisecond)
	require.Eventually(t, workerCancelled.Load, time.Second, time.Millisecond)
}

func TestNewSetsDependencies(t *testing.T) {
	t.Parallel()

	httpServer, queueWorker := newMocks(t)
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Second})

	assert.Same(t, httpServer, application.httpServer)
	assert.Same(t, queueWorker, application.queueWorker)
	assert.Equal(t, time.Second, application.shutdownTimeout)
}

func TestRunRequiresProcesses(t *testing.T) {
	t.Parallel()

	t.Run("missing HTTP server", func(t *testing.T) {
		err := (&App{}).Run(context.Background())

		require.Error(t, err)
		assert.EqualError(t, err, "http server is required")
	})

	t.Run("missing queue worker", func(t *testing.T) {
		httpServer, _ := newMocks(t)
		err := (&App{httpServer: httpServer}).Run(context.Background())

		require.Error(t, err)
		assert.EqualError(t, err, "queue worker is required")
	})
}

func TestRunReturnsHTTPServerError(t *testing.T) {
	t.Parallel()

	httpServer, queueWorker := newMocks(t)
	httpServer.EXPECT().ListenAndServe().Return(errors.New("boom"))
	httpServer.EXPECT().Shutdown(gomock.Any()).Return(nil)
	queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		<-mockCtx.Done()
		return nil
	})
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Second})

	err := application.Run(context.Background())
	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestRunReturnsQueueWorkerError(t *testing.T) {
	t.Parallel()

	shutdown := make(chan struct{})
	httpServer, queueWorker := newMocks(t)
	httpServer.EXPECT().ListenAndServe().DoAndReturn(func() error {
		<-shutdown
		return nil
	})
	httpServer.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		close(shutdown)
		return nil
	})
	queueWorker.EXPECT().Run(gomock.Any()).Return(errors.New("boom"))
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Second})

	err := application.Run(context.Background())
	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestRunReturnsNilWhenComponentStopsCleanly(t *testing.T) {
	t.Parallel()

	httpServer, queueWorker := newMocks(t)
	httpServer.EXPECT().ListenAndServe().Return(nil)
	httpServer.EXPECT().Shutdown(gomock.Any()).Return(nil)
	queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		<-mockCtx.Done()
		return nil
	})
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Second})

	require.NoError(t, application.Run(context.Background()))
}

func TestRunReturnsShutdownErrorAfterComponentFailure(t *testing.T) {
	t.Parallel()

	shutdown := make(chan struct{})
	httpServer, queueWorker := newMocks(t)
	httpServer.EXPECT().ListenAndServe().DoAndReturn(func() error {
		<-shutdown
		return nil
	})
	httpServer.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		close(shutdown)
		return errors.New("shutdown boom")
	})
	queueWorker.EXPECT().Run(gomock.Any()).Return(errors.New("worker boom"))
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Second})

	err := application.Run(context.Background())
	require.Error(t, err)
	assert.EqualError(t, err, "shutdown HTTP server: shutdown boom")
}

func TestRunIgnoresExpectedHTTPServerClosed(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	shutdown := make(chan struct{})
	httpServer, queueWorker := newMocks(t)
	httpServer.EXPECT().ListenAndServe().DoAndReturn(func() error {
		close(started)
		<-shutdown
		return http.ErrServerClosed
	})
	httpServer.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		close(shutdown)
		return nil
	})
	queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		<-mockCtx.Done()
		return nil
	})
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Millisecond})

	require.NoError(t, runWithCancel(t, application, ctx, cancel, started))
}

func TestRunIgnoresExpectedWorkerCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	shutdown := make(chan struct{})
	httpServer, queueWorker := newMocks(t)
	httpServer.EXPECT().ListenAndServe().DoAndReturn(func() error {
		close(started)
		<-shutdown
		return nil
	})
	httpServer.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		close(shutdown)
		return nil
	})
	queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		<-mockCtx.Done()
		return mockCtx.Err()
	})
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Millisecond})

	require.NoError(t, runWithCancel(t, application, ctx, cancel, started))
}

// A shutdown failure must win over cancellation so operators see the real stop
// error instead of a clean exit.
func TestRunReturnsShutdownError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	shutdown := make(chan struct{})
	httpServer, queueWorker := newMocks(t)
	httpServer.EXPECT().ListenAndServe().DoAndReturn(func() error {
		close(started)
		<-shutdown
		return nil
	})
	httpServer.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		close(shutdown)
		return errors.New("boom")
	})
	queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
		<-mockCtx.Done()
		return nil
	})
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Millisecond})
	err := runWithCancel(t, application, ctx, cancel, started)
	require.Error(t, err)
	assert.EqualError(t, err, "shutdown HTTP server: boom")
}

// Repeated single-instance shutdown runs give the race detector multiple real
// lifecycle interleavings without inventing unsupported multi-server usage.
func TestRunHandlesRepeatedConcurrentShutdownInterleavings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T, application *App, httpServer *appmock.MockHTTPRunner, queueWorker *appmock.MockQueueWorker)
	}{
		{
			name: "context cancellation",
			run: func(t *testing.T, application *App, httpServer *appmock.MockHTTPRunner, queueWorker *appmock.MockQueueWorker) {
				ctx, cancel := context.WithCancel(context.Background())
				started := make(chan struct{}, 2)
				shutdown := make(chan struct{})
				httpServer.EXPECT().ListenAndServe().DoAndReturn(func() error {
					started <- struct{}{}
					<-shutdown
					return nil
				})
				httpServer.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
					close(shutdown)
					return nil
				})
				queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
					started <- struct{}{}
					<-mockCtx.Done()
					return nil
				})

				err := runAfterStarts(t, application, ctx, func() {
					cancel()
				}, started, 2)
				require.NoError(t, err)
			},
		},
		{
			name: "worker stops first",
			run: func(t *testing.T, application *App, httpServer *appmock.MockHTTPRunner, queueWorker *appmock.MockQueueWorker) {
				ctx := context.Background()
				started := make(chan struct{}, 2)
				shutdown := make(chan struct{})
				workerRelease := make(chan struct{})
				httpServer.EXPECT().ListenAndServe().DoAndReturn(func() error {
					started <- struct{}{}
					<-shutdown
					return nil
				})
				httpServer.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
					close(shutdown)
					return nil
				})
				queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
					started <- struct{}{}
					<-workerRelease
					return errors.New("boom")
				})

				err := runAfterStarts(t, application, ctx, func() {
					close(workerRelease)
				}, started, 2)
				require.Error(t, err)
				assert.EqualError(t, err, "boom")
			},
		},
		{
			name: "http stops first",
			run: func(t *testing.T, application *App, httpServer *appmock.MockHTTPRunner, queueWorker *appmock.MockQueueWorker) {
				ctx := context.Background()
				started := make(chan struct{}, 2)
				httpRelease := make(chan struct{})
				httpServer.EXPECT().ListenAndServe().DoAndReturn(func() error {
					started <- struct{}{}
					<-httpRelease
					return errors.New("boom")
				})
				httpServer.EXPECT().Shutdown(gomock.Any()).Return(nil)
				queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(mockCtx context.Context) error {
					started <- struct{}{}
					<-mockCtx.Done()
					return nil
				})

				err := runAfterStarts(t, application, ctx, func() {
					close(httpRelease)
				}, started, 2)
				require.Error(t, err)
				assert.EqualError(t, err, "boom")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for range 100 {
				httpServer, queueWorker := newMocks(t)
				application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Millisecond})
				test.run(t, application, httpServer, queueWorker)
			}
		})
	}
}

func TestShutdownTimeoutUsesFallback(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 10*time.Second, shutdownTimeout(Options{}))
}
