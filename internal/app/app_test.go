package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStartsProcessesAndCancelsWorker(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	httpServer := newBlockingHTTPServer()
	queueWorker := &fakeQueueWorker{}
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Second})
	done := make(chan error, 1)

	go func() {
		done <- application.Run(ctx)
	}()

	<-httpServer.started
	cancel()

	require.NoError(t, <-done)
	require.Eventually(t, httpServer.shutdownCalled.Load, time.Second, time.Millisecond)
	require.Eventually(t, queueWorker.cancelled.Load, time.Second, time.Millisecond)
}

func TestNewSetsDependencies(t *testing.T) {
	t.Parallel()

	httpServer := &fakeHTTPServer{}
	queueWorker := &fakeQueueWorker{}
	application := New(httpServer, queueWorker, Options{ShutdownTimeout: time.Second})

	assert.Same(t, httpServer, application.httpServer)
	assert.Same(t, queueWorker, application.queueWorker)
	assert.Equal(t, time.Second, application.shutdownTimeout)
}

func TestRunRequiresProcesses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		application *App
		wantErr     string
	}{
		{name: "missing HTTP server", application: &App{}, wantErr: "http server is required"},
		{name: "missing queue worker", application: &App{httpServer: &fakeHTTPServer{}}, wantErr: "queue worker is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.application.Run(context.Background())

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
}

func TestRunReturnsHTTPServerError(t *testing.T) {
	t.Parallel()

	application := &App{
		httpServer:      &fakeHTTPServer{listenErr: errors.New("boom")},
		queueWorker:     &fakeQueueWorker{},
		shutdownTimeout: time.Second,
	}

	err := application.Run(context.Background())
	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestRunReturnsQueueWorkerError(t *testing.T) {
	t.Parallel()

	application := &App{
		httpServer:      newBlockingHTTPServer(),
		queueWorker:     &fakeQueueWorker{err: errors.New("boom")},
		shutdownTimeout: time.Second,
	}

	err := application.Run(context.Background())
	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestRunReturnsNilWhenComponentStopsCleanly(t *testing.T) {
	t.Parallel()

	application := &App{
		httpServer:      &fakeHTTPServer{},
		queueWorker:     &fakeQueueWorker{},
		shutdownTimeout: time.Second,
	}

	require.NoError(t, application.Run(context.Background()))
}

func TestRunReturnsShutdownErrorAfterComponentFailure(t *testing.T) {
	t.Parallel()

	application := &App{
		httpServer:      &fakeHTTPServer{shutdownErr: errors.New("shutdown boom")},
		queueWorker:     &fakeQueueWorker{err: errors.New("worker boom")},
		shutdownTimeout: time.Second,
	}

	err := application.Run(context.Background())
	require.Error(t, err)
	assert.EqualError(t, err, "shutdown HTTP server: shutdown boom")
}

// Cancellation is the daemon's normal shutdown path, so this test keeps the
// HTTP and worker teardown contract explicit.
func TestRunShutsDownHTTPServerOnContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	httpServer := newBlockingHTTPServer()
	queueWorker := &fakeQueueWorker{}
	application := &App{
		httpServer:      httpServer,
		queueWorker:     queueWorker,
		shutdownTimeout: time.Millisecond,
	}
	done := make(chan error, 1)

	go func() {
		done <- application.Run(ctx)
	}()

	<-httpServer.started
	cancel()

	require.NoError(t, <-done)
	require.Eventually(t, httpServer.shutdownCalled.Load, time.Second, time.Millisecond)
	require.Eventually(t, queueWorker.cancelled.Load, time.Second, time.Millisecond)
}

// A shutdown failure must win over cancellation so operators see the real stop
// error instead of a clean exit.
func TestRunReturnsShutdownError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	httpServer := newBlockingHTTPServer()
	httpServer.shutdownErr = errors.New("boom")
	application := &App{
		httpServer:      httpServer,
		queueWorker:     &fakeQueueWorker{},
		shutdownTimeout: time.Millisecond,
	}
	done := make(chan error, 1)

	go func() {
		done <- application.Run(ctx)
	}()

	<-httpServer.started
	cancel()

	err := <-done
	require.Error(t, err)
	assert.EqualError(t, err, "shutdown HTTP server: boom")
}

func TestShutdownTimeoutUsesFallback(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 10*time.Second, shutdownTimeout(Options{}))
}

type fakeHTTPServer struct {
	mu             sync.Mutex
	started        chan struct{}
	startedOnce    sync.Once
	shutdown       chan struct{}
	shutdownOnce   sync.Once
	listenBlocks   bool
	listenErr      error
	shutdownCalled atomic.Bool
	shutdownErr    error
}

func newBlockingHTTPServer() *fakeHTTPServer {
	return &fakeHTTPServer{
		started:      make(chan struct{}),
		shutdown:     make(chan struct{}),
		listenBlocks: true,
	}
}

func (server *fakeHTTPServer) ListenAndServe() error {
	server.startedOnce.Do(func() {
		close(server.startedChan())
	})
	if server.listenBlocks {
		<-server.shutdownChan()
	}

	return server.listenErr
}

func (server *fakeHTTPServer) Shutdown(ctx context.Context) error {
	server.shutdownCalled.Store(true)
	server.shutdownOnce.Do(func() {
		close(server.shutdownChan())
	})
	return server.shutdownErr
}

// startedChan returns the startup signal channel.
func (server *fakeHTTPServer) startedChan() chan struct{} {
	server.mu.Lock()
	defer server.mu.Unlock()

	if server.started == nil {
		server.started = make(chan struct{})
	}

	return server.started
}

// shutdownChan returns the shutdown signal channel.
func (server *fakeHTTPServer) shutdownChan() chan struct{} {
	server.mu.Lock()
	defer server.mu.Unlock()

	if server.shutdown == nil {
		server.shutdown = make(chan struct{})
	}

	return server.shutdown
}

type fakeQueueWorker struct {
	err       error
	cancelled atomic.Bool
}

func (worker *fakeQueueWorker) Run(ctx context.Context) error {
	if worker.err != nil {
		return worker.err
	}
	<-ctx.Done()
	worker.cancelled.Store(true)
	return nil
}
