package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestRunReturnsDefaultWiringError(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), config.Config{})

	require.Error(t, err)
	assert.EqualError(t, err, "create HTTP server: validate HTTP dependencies: message table is required")
}

func TestRunWithRequiresFactory(t *testing.T) {
	t.Parallel()

	err := RunWith(context.Background(), config.Config{}, Options{})

	require.Error(t, err)
	assert.EqualError(t, err, "factory is required")
}

func TestRunWithReturnsHTTPFactoryError(t *testing.T) {
	t.Parallel()

	err := RunWith(context.Background(), config.Config{}, Options{Factory: &fakeFactory{httpErr: errors.New("boom")}})

	require.Error(t, err)
	assert.EqualError(t, err, "create HTTP server: boom")
}

func TestRunWithReturnsQueueFactoryError(t *testing.T) {
	t.Parallel()

	err := RunWith(context.Background(), config.Config{}, Options{Factory: &fakeFactory{queueErr: errors.New("boom")}})

	require.Error(t, err)
	assert.EqualError(t, err, "create queue worker: boom")
}

func TestRunWithReturnsHTTPServerError(t *testing.T) {
	t.Parallel()

	err := RunWith(context.Background(), config.Config{}, Options{Factory: &fakeFactory{
		httpServer: &fakeHTTPServer{listenErr: errors.New("boom")},
	}})

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestRunWithReturnsQueueWorkerError(t *testing.T) {
	t.Parallel()

	err := RunWith(context.Background(), config.Config{}, Options{Factory: &fakeFactory{
		httpServer:  newBlockingHTTPServer(),
		queueWorker: &fakeQueueWorker{err: errors.New("boom")},
	}})

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestRunWithReturnsNilWhenComponentStopsCleanly(t *testing.T) {
	t.Parallel()

	err := RunWith(context.Background(), config.Config{}, Options{Factory: &fakeFactory{
		httpServer:  &fakeHTTPServer{},
		queueWorker: &fakeQueueWorker{},
	}})

	require.NoError(t, err)
}

func TestRunWithReturnsShutdownErrorAfterComponentFailure(t *testing.T) {
	t.Parallel()

	err := RunWith(context.Background(), config.Config{}, Options{Factory: &fakeFactory{
		httpServer:  &fakeHTTPServer{shutdownErr: errors.New("shutdown boom")},
		queueWorker: &fakeQueueWorker{err: errors.New("worker boom")},
	}})

	require.Error(t, err)
	assert.EqualError(t, err, "shutdown HTTP server: shutdown boom")
}

func TestRunWithShutsDownHTTPServerOnContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	httpServer := newBlockingHTTPServer()
	queueWorker := &fakeQueueWorker{}
	done := make(chan error, 1)

	go func() {
		done <- RunWith(ctx, config.Config{}, Options{
			Factory:         &fakeFactory{httpServer: httpServer, queueWorker: queueWorker},
			ShutdownTimeout: time.Millisecond,
		})
	}()

	<-httpServer.started
	cancel()

	require.NoError(t, <-done)
	assert.True(t, httpServer.shutdownCalled)
	assert.True(t, queueWorker.cancelled)
}

func TestRunWithReturnsShutdownError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	httpServer := newBlockingHTTPServer()
	httpServer.shutdownErr = errors.New("boom")
	done := make(chan error, 1)

	go func() {
		done <- RunWith(ctx, config.Config{ShutdownTimeout: time.Millisecond}, Options{
			Factory: &fakeFactory{httpServer: httpServer, queueWorker: &fakeQueueWorker{}},
		})
	}()

	<-httpServer.started
	cancel()

	err := <-done
	require.Error(t, err)
	assert.EqualError(t, err, "shutdown HTTP server: boom")
}

func TestRunWithUsesFallbackShutdownTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 10*time.Second, shutdownTimeout(config.Config{}, Options{}))
}

func TestRuntimeFactoryBuildsHTTPServer(t *testing.T) {
	t.Parallel()

	server, err := (RuntimeFactory{Store: &runtimeStore{}, Sender: &runtimeSender{}}).NewHTTPServer(runtimeConfig())

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

func TestRuntimeFactoryBuildsQueueWorker(t *testing.T) {
	t.Parallel()

	queueWorker, err := (RuntimeFactory{Receiver: &runtimeReceiver{}, Deleter: &runtimeDeleter{}}).NewQueueWorker(runtimeConfig())

	require.NoError(t, err)
	_, ok := queueWorker.(worker.PollingWorker)
	assert.True(t, ok)
}

func TestRuntimeFactoryQueueWorkerRequiresReceiver(t *testing.T) {
	t.Parallel()

	_, err := (RuntimeFactory{Deleter: &runtimeDeleter{}}).NewQueueWorker(runtimeConfig())

	require.Error(t, err)
	assert.EqualError(t, err, "queue receiver is required")
}

func TestRuntimeFactoryQueueWorkerRequiresDeleter(t *testing.T) {
	t.Parallel()

	_, err := (RuntimeFactory{Receiver: &runtimeReceiver{}}).NewQueueWorker(runtimeConfig())

	require.Error(t, err)
	assert.EqualError(t, err, "queue deleter is required")
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
	started        chan struct{}
	shutdown       chan struct{}
	listenBlocks   bool
	listenErr      error
	shutdownCalled bool
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
	if server.started == nil {
		server.started = make(chan struct{})
	}
	if server.shutdown == nil {
		server.shutdown = make(chan struct{})
	}
	close(server.started)
	if server.listenBlocks {
		<-server.shutdown
	}

	return server.listenErr
}

func (server *fakeHTTPServer) Shutdown(ctx context.Context) error {
	server.shutdownCalled = true
	if server.shutdown != nil {
		close(server.shutdown)
	}
	return server.shutdownErr
}

type fakeQueueWorker struct {
	err       error
	cancelled bool
}

func (worker *fakeQueueWorker) Run(ctx context.Context) error {
	if worker.err != nil {
		return worker.err
	}
	<-ctx.Done()
	worker.cancelled = true
	return nil
}

func runtimeConfig() config.Config {
	return config.Config{
		MessageTable:    "messages",
		ChatIDTable:     "chats",
		EventQueueURL:   "queue-url",
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
