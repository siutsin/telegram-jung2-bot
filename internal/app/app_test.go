package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

func TestNewBuildsAndStartsRuntime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	httpServer := newBlockingHTTPServer()
	queueWorker := &fakeQueueWorker{}
	application := &App{
		httpServer:      httpServer,
		queueWorker:     queueWorker,
		shutdownTimeout: time.Second,
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

func TestNewBuildsHTTPServer(t *testing.T) {
	t.Parallel()

	application, err := New(runtimeConfig(), runtimeDependencies(), Options{})

	require.NoError(t, err)
	httpServer, ok := application.httpServer.(*http.Server)
	require.True(t, ok)
	assert.Equal(t, ":3000", httpServer.Addr)
	assert.Equal(t, 5*time.Second, httpServer.ReadTimeout)

	response := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestNewRejectsInvalidHTTPDependencies(t *testing.T) {
	t.Parallel()

	_, err := New(runtimeConfig(), Dependencies{}, Options{})

	require.Error(t, err)
	assert.EqualError(t, err, "create HTTP server: validate HTTP dependencies: message store is required")
}

func TestNewBuildsScaleUpRoute(t *testing.T) {
	t.Parallel()

	dependencies := runtimeDependencies()
	dependencies.ScaleUpper = &runtimeScaleUpper{}
	application, err := New(runtimeConfig(), dependencies, Options{})

	require.NoError(t, err)
	httpServer, ok := application.httpServer.(*http.Server)
	require.True(t, ok)
	response := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/jung2bot/dev/onScaleUp", nil))
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestNewBuildsQueueWorker(t *testing.T) {
	t.Parallel()

	application, err := New(runtimeConfig(), runtimeDependencies(), Options{})

	require.NoError(t, err)
	_, ok := application.queueWorker.(worker.PollingWorker)
	assert.True(t, ok)
}

func TestNewQueueWorkerRequiresDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dependencies Dependencies
		wantErr      string
	}{
		{
			name: "missing receiver",
			dependencies: Dependencies{
				Chats:     &runtimeChatStore{},
				Messages:  &runtimeMessageStore{},
				Sender:    &runtimeSender{},
				Deleter:   &runtimeDeleter{},
				Messenger: &runtimeMessenger{},
			},
			wantErr: "create queue worker: queue receiver is required",
		},
		{
			name: "missing deleter",
			dependencies: Dependencies{
				Chats:     &runtimeChatStore{},
				Messages:  &runtimeMessageStore{},
				Sender:    &runtimeSender{},
				Receiver:  &runtimeReceiver{},
				Messenger: &runtimeMessenger{},
			},
			wantErr: "create queue worker: queue deleter is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(runtimeConfig(), test.dependencies, Options{})

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
}

func TestRunWithSetupErrors(t *testing.T) {
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

	assert.Equal(t, 10*time.Second, shutdownTimeout(config.Config{}, Options{}))
}

func runtimeConfig() config.Config {
	return config.Config{
		MessageTable:    "messages",
		ChatIDTable:     "chats",
		EventQueueURL:   "queue-url",
		Stage:           "dev",
		ServerAddress:   ":3000",
		HTTPTimeout:     5 * time.Second,
		ShutdownTimeout: time.Second,
	}
}

func runtimeDependencies() Dependencies {
	return Dependencies{
		Chats:     &runtimeChatStore{},
		Messages:  &runtimeMessageStore{},
		Sender:    &runtimeSender{},
		Receiver:  &runtimeReceiver{},
		Deleter:   &runtimeDeleter{},
		Messenger: &runtimeMessenger{},
	}
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

type runtimeMessageStore struct{}

func (store *runtimeMessageStore) Save(ctx context.Context, tableName string, row message.Message) error {
	return nil
}

type runtimeChatStore struct{}

func (store *runtimeChatStore) Save(ctx context.Context, tableName string, settings chat.ChatSetting) error {
	return nil
}

type runtimeSender struct{}

func (sender *runtimeSender) SendMessage(ctx context.Context, request queue.SendMessageRequest) error {
	return nil
}

type runtimeReceiver struct{}

func (receiver *runtimeReceiver) ReceiveMessage(ctx context.Context, request queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error) {
	return queue.ReceiveMessageResponse{}, nil
}

type runtimeDeleter struct{}

func (deleter *runtimeDeleter) Delete(ctx context.Context, request queue.DeleteMessageRequest) error {
	return nil
}

type runtimeMessenger struct{}

func (messenger *runtimeMessenger) SendMessage(ctx context.Context, chatID int64, text string) error {
	return nil
}

type runtimeScaleUpper struct{}

func (scaleUpper *runtimeScaleUpper) ScaleUp(ctx context.Context) error {
	return nil
}
