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

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBuildsAndStartsDefaultRuntime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	httpServer := newBlockingHTTPServer()
	queueWorker := &fakeQueueWorker{}
	application, err := New(ctx, config.Config{}, Options{FactoryBuilder: func(ctx context.Context, config config.Config) (Factory, error) {
		return &fakeFactory{httpServer: httpServer, queueWorker: queueWorker}, nil
	}})
	require.NoError(t, err)
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

func TestRunReturnsRuntimeFactoryError(t *testing.T) {
	_, err := New(context.Background(), config.Config{}, Options{FactoryBuilder: func(ctx context.Context, config config.Config) (Factory, error) {
		return nil, errors.New("boom")
	}})

	require.Error(t, err)
	assert.EqualError(t, err, "build runtime factory: boom")
}

func TestNewUsesProvidedFactory(t *testing.T) {
	t.Parallel()

	application, err := New(context.Background(), config.Config{ShutdownTimeout: time.Second}, Options{
		Factory:         &fakeFactory{},
		ShutdownTimeout: 2 * time.Second,
	})

	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, application.shutdownTimeout)
}

// These cases pin the setup-stage error boundaries so wiring regressions fail
// before the service starts goroutines.
func TestRunWithSetupErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		application *App
		wantErr     string
	}{
		{name: "missing factory", application: &App{}, wantErr: "factory is required"},
		{name: "http factory error", application: &App{factory: &fakeFactory{httpErr: errors.New("boom")}}, wantErr: "create HTTP server: boom"},
		{name: "queue factory error", application: &App{factory: &fakeFactory{queueErr: errors.New("boom")}}, wantErr: "create queue worker: boom"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.application.Run(context.Background())

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
}

func TestRunWithReturnsHTTPServerError(t *testing.T) {
	t.Parallel()

	application, err := New(context.Background(), config.Config{}, Options{
		Factory: &fakeFactory{httpServer: &fakeHTTPServer{listenErr: errors.New("boom")}},
	})
	require.NoError(t, err)

	err = application.Run(context.Background())
	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestRunWithReturnsQueueWorkerError(t *testing.T) {
	t.Parallel()

	application, err := New(context.Background(), config.Config{}, Options{Factory: &fakeFactory{
		httpServer:  newBlockingHTTPServer(),
		queueWorker: &fakeQueueWorker{err: errors.New("boom")},
	}})
	require.NoError(t, err)

	err = application.Run(context.Background())
	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestRunWithReturnsNilWhenComponentStopsCleanly(t *testing.T) {
	t.Parallel()

	application, err := New(context.Background(), config.Config{}, Options{Factory: &fakeFactory{
		httpServer:  &fakeHTTPServer{},
		queueWorker: &fakeQueueWorker{},
	}})
	require.NoError(t, err)

	require.NoError(t, application.Run(context.Background()))
}

func TestRunWithReturnsShutdownErrorAfterComponentFailure(t *testing.T) {
	t.Parallel()

	application, err := New(context.Background(), config.Config{}, Options{Factory: &fakeFactory{
		httpServer:  &fakeHTTPServer{shutdownErr: errors.New("shutdown boom")},
		queueWorker: &fakeQueueWorker{err: errors.New("worker boom")},
	}})
	require.NoError(t, err)

	err = application.Run(context.Background())
	require.Error(t, err)
	assert.EqualError(t, err, "shutdown HTTP server: shutdown boom")
}

// Cancellation is the daemon's normal shutdown path, so this test keeps the
// HTTP and worker teardown contract explicit.
func TestRunWithShutsDownHTTPServerOnContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	httpServer := newBlockingHTTPServer()
	queueWorker := &fakeQueueWorker{}
	application, err := New(context.Background(), config.Config{}, Options{
		Factory:         &fakeFactory{httpServer: httpServer, queueWorker: queueWorker},
		ShutdownTimeout: time.Millisecond,
	})
	require.NoError(t, err)
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
func TestRunWithReturnsShutdownError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	httpServer := newBlockingHTTPServer()
	httpServer.shutdownErr = errors.New("boom")
	application, err := New(context.Background(), config.Config{ShutdownTimeout: time.Millisecond}, Options{
		Factory: &fakeFactory{httpServer: httpServer, queueWorker: &fakeQueueWorker{}},
	})
	require.NoError(t, err)
	done := make(chan error, 1)

	go func() {
		done <- application.Run(ctx)
	}()

	<-httpServer.started
	cancel()

	err = <-done
	require.Error(t, err)
	assert.EqualError(t, err, "shutdown HTTP server: boom")
}

func TestRunWithUsesFallbackShutdownTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 10*time.Second, shutdownTimeout(config.Config{}, Options{}))
}

func TestRuntimeFactoryBuildsHTTPServer(t *testing.T) {
	t.Parallel()

	server, err := (RuntimeFactory{Messenger: &runtimeMessenger{}, Store: &runtimeStore{}, Sender: &runtimeSender{}}).NewHTTPServer(runtimeConfig())

	require.NoError(t, err)
	httpServer, ok := server.(*http.Server)
	require.True(t, ok)
	assert.Equal(t, ":3000", httpServer.Addr)
	assert.Equal(t, 5*time.Second, httpServer.ReadTimeout)

	response := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestRuntimeFactoryRejectsInvalidHTTPDependencies(t *testing.T) {
	t.Parallel()

	_, err := (RuntimeFactory{}).NewHTTPServer(runtimeConfig())

	require.Error(t, err)
	assert.EqualError(t, err, "validate HTTP dependencies: store is required")
}

func TestRuntimeFactoryBuildsScaleUpRoute(t *testing.T) {
	t.Parallel()

	server, err := (RuntimeFactory{
		Messenger:  &runtimeMessenger{},
		ScaleUpper: &runtimeScaleUpper{},
		Store:      &runtimeStore{},
		Sender:     &runtimeSender{},
	}).NewHTTPServer(runtimeConfig())

	require.NoError(t, err)
	httpServer := server.(*http.Server)
	response := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onScaleUp", nil))
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestRuntimeFactoryBuildsQueueWorker(t *testing.T) {
	t.Parallel()

	queueWorker, err := (RuntimeFactory{Receiver: &runtimeReceiver{}, Deleter: &runtimeDeleter{}}).NewQueueWorker(runtimeConfig())

	require.NoError(t, err)
	_, ok := queueWorker.(worker.PollingWorker)
	assert.True(t, ok)
}

func TestRuntimeFactoryQueueWorkerRequiresDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		factory RuntimeFactory
		wantErr string
	}{
		{name: "missing receiver", factory: RuntimeFactory{Deleter: &runtimeDeleter{}}, wantErr: "queue receiver is required"},
		{name: "missing deleter", factory: RuntimeFactory{Receiver: &runtimeReceiver{}}, wantErr: "queue deleter is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.factory.NewQueueWorker(runtimeConfig())

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
}

type fakeFactory struct {
	httpServer  HTTPServer
	queueWorker QueueWorker
	httpErr     error
	queueErr    error
}

func (factory *fakeFactory) NewHTTPServer(config config.Config) (HTTPServer, error) {
	if factory.httpErr != nil {
		return nil, factory.httpErr
	}
	if factory.httpServer != nil {
		return factory.httpServer, nil
	}

	return &fakeHTTPServer{}, nil
}

func (factory *fakeFactory) NewQueueWorker(config config.Config) (QueueWorker, error) {
	if factory.queueErr != nil {
		return nil, factory.queueErr
	}
	if factory.queueWorker != nil {
		return factory.queueWorker, nil
	}

	return &fakeQueueWorker{}, nil
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

type runtimeStore struct{}

func (store *runtimeStore) SaveMessage(ctx context.Context, request message.UpdateExpression) error {
	return nil
}

func (store *runtimeStore) SaveChat(ctx context.Context, request chat.UpdateExpression) error {
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
