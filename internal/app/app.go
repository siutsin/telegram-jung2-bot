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
	"github.com/siutsin/telegram-jung2-bot/internal/runtime"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

type HTTPServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type QueueWorker interface {
	Run(ctx context.Context) error
}

type Factory interface {
	NewHTTPServer(config config.Config) (HTTPServer, error)
	NewQueueWorker(config config.Config) (QueueWorker, error)
}

// FactoryBuilder constructs a runtime factory for one configured app instance.
type FactoryBuilder func(ctx context.Context, config config.Config) (Factory, error)

// App wraps the configured application runtime and its dependencies.
type App struct {
	config          config.Config
	factory         Factory
	shutdownTimeout time.Duration
}

// Options configures how an application instance is assembled.
type Options struct {
	Factory         Factory
	FactoryBuilder  FactoryBuilder
	ShutdownTimeout time.Duration
}

type RuntimeFactory struct {
	Store      httpserver.Store
	Sender     queue.Sender
	Receiver   queue.Receiver
	Deleter    worker.Deleter
	Messenger  httpserver.Messenger
	ScaleUpper httpserver.ScaleUpper
	Handlers   worker.Handlers
	Now        func() time.Time
}

// New constructs an application with the provided runtime options.
func New(ctx context.Context, config config.Config, options Options) (*App, error) {
	factory, err := appFactory(ctx, config, options)
	if err != nil {
		return nil, err
	}

	return &App{
		config:          config,
		factory:         factory,
		shutdownTimeout: shutdownTimeout(config, options),
	}, nil
}

// Run starts the configured application.
func (app *App) Run(ctx context.Context) error {
	if app == nil || app.factory == nil {
		return fmt.Errorf("factory is required")
	}

	httpServer, err := app.factory.NewHTTPServer(app.config)
	if err != nil {
		return fmt.Errorf("create HTTP server: %w", err)
	}
	queueWorker, err := app.factory.NewQueueWorker(app.config)
	if err != nil {
		return fmt.Errorf("create queue worker: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, 2)
	go func() {
		errs <- httpServer.ListenAndServe()
	}()
	go func() {
		errs <- queueWorker.Run(runCtx)
	}()

	select {
	case <-ctx.Done():
		cancel()
		return shutdownHTTP(httpServer, app.shutdownTimeout)
	case err := <-errs:
		cancel()
		if shutdownErr := shutdownHTTP(httpServer, app.shutdownTimeout); shutdownErr != nil {
			return shutdownErr
		}
		if err != nil {
			return err
		}
		return nil
	}
}

// appFactory resolves the factory to use for a configured application.
func appFactory(ctx context.Context, config config.Config, options Options) (Factory, error) {
	if options.Factory != nil {
		return options.Factory, nil
	}

	builder := options.FactoryBuilder
	if builder == nil {
		builder = buildRuntimeFactory
	}

	factory, err := builder(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("build runtime factory: %w", err)
	}

	return factory, nil
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

// NewHTTPServer builds the app HTTP server.
func (factory RuntimeFactory) NewHTTPServer(config config.Config) (HTTPServer, error) {
	dependencies := httpserver.Dependencies{
		MessageTable: config.MessageTable,
		ChatTable:    config.ChatIDTable,
		Store:        factory.Store,
		Enqueuer:     queue.Producer{QueueURL: config.EventQueueURL, Sender: factory.Sender},
		Messenger:    factory.Messenger,
		ScaleUpper:   factory.ScaleUpper,
		Now:          factory.Now,
	}
	if err := httpserver.Validate(dependencies); err != nil {
		return nil, fmt.Errorf("validate HTTP dependencies: %w", err)
	}

	return &http.Server{
		Addr:              config.ServerAddress,
		Handler:           httpserver.New(httpserver.ServerDeps{Dependencies: dependencies, Stage: config.Stage}),
		ReadHeaderTimeout: config.HTTPTimeout,
		ReadTimeout:       config.HTTPTimeout,
		WriteTimeout:      config.HTTPTimeout,
		IdleTimeout:       config.HTTPTimeout,
	}, nil
}

// NewQueueWorker builds the app queue worker.
func (factory RuntimeFactory) NewQueueWorker(config config.Config) (QueueWorker, error) {
	if factory.Receiver == nil {
		return nil, fmt.Errorf("queue receiver is required")
	}
	if factory.Deleter == nil {
		return nil, fmt.Errorf("queue deleter is required")
	}

	return worker.PollingWorker{
		Consumer: queue.Consumer{QueueURL: config.EventQueueURL, Receiver: factory.Receiver},
		QueueURL: config.EventQueueURL,
		Handlers: factory.Handlers,
		Deleter:  factory.Deleter,
	}, nil
}

// buildRuntimeFactory assembles the production runtime factory.
func buildRuntimeFactory(ctx context.Context, config config.Config) (Factory, error) {
	components, err := runtime.NewComponents(ctx, config)
	if err != nil {
		return nil, err
	}

	return RuntimeFactory{
		Store:      components.Store,
		Sender:     components.Sender,
		Receiver:   components.Receiver,
		Deleter:    components.Deleter,
		Messenger:  components.Messenger,
		ScaleUpper: components.ScaleUpper,
		Handlers:   components.Handlers,
		Now:        components.Now,
	}, nil
}
