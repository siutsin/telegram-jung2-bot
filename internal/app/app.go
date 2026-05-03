// Package app owns service lifecycle orchestration.
package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/config"
	"github.com/siutsin/telegram-jung2-bot/internal/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

type HTTPServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type QueueWorker interface {
	Run(ctx context.Context) error
}

// App wraps the configured application processes and dependencies.
type App struct {
	httpServer      HTTPServer
	queueWorker     QueueWorker
	shutdownTimeout time.Duration
}

// Options configures how an application instance is assembled.
type Options struct {
	ShutdownTimeout time.Duration
}

// Dependencies contains the collaborators the app needs.
type Dependencies struct {
	Chats      httpserver.ChatStore
	Messages   httpserver.MessageStore
	Sender     queue.Sender
	Receiver   queue.Receiver
	Deleter    worker.Deleter
	Messenger  httpserver.Messenger
	ScaleUpper httpserver.ScaleUpper
	Handlers   worker.Handlers
	Now        func() time.Time
}

// New constructs an application with the provided dependencies and options.
func New(config config.Config, dependencies Dependencies, options Options) (*App, error) {
	httpServer, err := newHTTPServer(config, dependencies)
	if err != nil {
		return nil, fmt.Errorf("create HTTP server: %w", err)
	}
	queueWorker, err := newQueueWorker(config, dependencies)
	if err != nil {
		return nil, fmt.Errorf("create queue worker: %w", err)
	}

	return &App{
		httpServer:      httpServer,
		queueWorker:     queueWorker,
		shutdownTimeout: shutdownTimeout(config, options),
	}, nil
}

// Run starts the configured application.
func (app *App) Run(ctx context.Context) error {
	if app == nil || app.httpServer == nil {
		return fmt.Errorf("http server is required")
	}
	if app.queueWorker == nil {
		return fmt.Errorf("queue worker is required")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, 2)
	go func() {
		errs <- app.httpServer.ListenAndServe()
	}()
	go func() {
		errs <- app.queueWorker.Run(runCtx)
	}()

	select {
	case <-ctx.Done():
		cancel()
		return shutdownHTTP(app.httpServer, app.shutdownTimeout)
	case err := <-errs:
		cancel()
		if shutdownErr := shutdownHTTP(app.httpServer, app.shutdownTimeout); shutdownErr != nil {
			return shutdownErr
		}
		if err != nil {
			return err
		}
		return nil
	}
}

// shutdownHTTP stops the HTTP server with a timeout.
func shutdownHTTP(httpServer HTTPServer, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	return nil
}

// shutdownTimeout returns the configured shutdown timeout.
func shutdownTimeout(config config.Config, options Options) time.Duration {
	if options.ShutdownTimeout > 0 {
		return options.ShutdownTimeout
	}
	if config.ShutdownTimeout > 0 {
		return config.ShutdownTimeout
	}

	return 10 * time.Second
}

// newHTTPServer builds the app HTTP server.
func newHTTPServer(config config.Config, dependencies Dependencies) (HTTPServer, error) {
	httpDependencies := httpserver.Dependencies{
		Chats:      dependencies.Chats,
		Messages:   dependencies.Messages,
		Enqueuer:   queue.Producer{QueueURL: config.EventQueueURL, Sender: dependencies.Sender},
		Messenger:  dependencies.Messenger,
		ScaleUpper: dependencies.ScaleUpper,
		Now:        dependencies.Now,
	}
	if err := httpserver.Validate(httpDependencies); err != nil {
		return nil, fmt.Errorf("validate HTTP dependencies: %w", err)
	}

	return &http.Server{
		Addr:              config.ServerAddress,
		Handler:           httpserver.New(httpserver.ServerDeps{Dependencies: httpDependencies, Stage: config.Stage}),
		ReadHeaderTimeout: config.HTTPTimeout,
		ReadTimeout:       config.HTTPTimeout,
		WriteTimeout:      config.HTTPTimeout,
		IdleTimeout:       config.HTTPTimeout,
	}, nil
}

// newQueueWorker builds the app queue worker.
func newQueueWorker(config config.Config, dependencies Dependencies) (QueueWorker, error) {
	if dependencies.Receiver == nil {
		return nil, fmt.Errorf("queue receiver is required")
	}
	if dependencies.Deleter == nil {
		return nil, fmt.Errorf("queue deleter is required")
	}

	return worker.PollingWorker{
		Consumer: queue.Consumer{QueueURL: config.EventQueueURL, Receiver: dependencies.Receiver},
		QueueURL: config.EventQueueURL,
		Handlers: dependencies.Handlers,
		Deleter:  dependencies.Deleter,
	}, nil
}
