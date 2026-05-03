package bootstrap

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/siutsin/telegram-jung2-bot/internal/app"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Keep this as a subprocess test because main exits non-zero on fatal startup
// errors, which would terminate the parent test process.
func TestMainExitsNonZeroOnStartupError(t *testing.T) {
	t.Parallel()

	command := exec.Command("go", "run", "./cmd")
	command.Dir = "../.."
	err := command.Run()

	require.Error(t, err)
	exitError, ok := err.(*exec.ExitError)
	require.True(t, ok)
	assert.NotZero(t, exitError.ExitCode())
}

// This guards the handoff from environment parsing into app startup so missing
// config changes fail here instead of silently shifting runtime wiring tests.
func TestRunLoadsEnvironment(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("sentinel")
	called := false
	err := Run(context.Background(), []string{
		"TELEGRAM_BOT_TOKEN=token",
		"MESSAGE_TABLE=messages",
		"CHATID_TABLE=chats",
		"EVENT_QUEUE_URL=https://sqs.eu-west-1.amazonaws.com/123/events",
	}, Options{
		AppOptions: app.Options{
			Factory: &bootstrapFactory{
				httpServer:  &bootstrapHTTPServer{listenErr: sentinel},
				queueWorker: &bootstrapQueueWorker{},
				onHTTPConfig: func(loaded config.Config) {
					called = true
					assert.Equal(t, "token", loaded.TelegramBotToken)
					assert.Equal(t, "messages", loaded.MessageTable)
					assert.Equal(t, "chats", loaded.ChatIDTable)
					assert.Equal(t, "https://sqs.eu-west-1.amazonaws.com/123/events", loaded.EventQueueURL)
				},
			},
		},
	})

	require.Error(t, err)
	assert.EqualError(t, err, "sentinel")
	assert.True(t, called)
}

func TestRunReturnsConfigError(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), nil, Options{})

	require.Error(t, err)
	assert.EqualError(t, err, "TELEGRAM_BOT_TOKEN is required")
}

type bootstrapFactory struct {
	httpServer   app.HTTPServer
	queueWorker  app.QueueWorker
	onHTTPConfig func(config.Config)
}

// NewHTTPServer builds the test HTTP server.
func (factory *bootstrapFactory) NewHTTPServer(loaded config.Config) (app.HTTPServer, error) {
	if factory.onHTTPConfig != nil {
		factory.onHTTPConfig(loaded)
	}

	return factory.httpServer, nil
}

// NewQueueWorker builds the test queue worker.
func (factory *bootstrapFactory) NewQueueWorker(loaded config.Config) (app.QueueWorker, error) {
	return factory.queueWorker, nil
}

type bootstrapHTTPServer struct {
	listenErr error
}

// ListenAndServe runs the test HTTP server.
func (server *bootstrapHTTPServer) ListenAndServe() error {
	return server.listenErr
}

// Shutdown stops the test HTTP server.
func (server *bootstrapHTTPServer) Shutdown(ctx context.Context) error {
	return nil
}

type bootstrapQueueWorker struct{}

// Run blocks until startup cancellation reaches the test worker.
func (worker *bootstrapQueueWorker) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
