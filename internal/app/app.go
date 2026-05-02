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

type Factory interface {
	NewHTTPServer(config config.Config) (HTTPServer, error)
	NewQueueWorker(config config.Config) (QueueWorker, error)
}

type Options struct {
	Factory         Factory
	ShutdownTimeout time.Duration
}

type RuntimeFactory struct {
	Store     httpserver.Store
	Sender    queue.Sender
	Receiver  queue.Receiver
	Deleter   worker.Deleter
	Messenger httpserver.Messenger
	Handlers  worker.Handlers
	Now       func() time.Time
}

func Run(ctx context.Context, config config.Config) error {
	return RunWith(ctx, config, Options{
		Factory:         RuntimeFactory{},
		ShutdownTimeout: config.ShutdownTimeout,
	})
}

func RunWith(ctx context.Context, config config.Config, options Options) error {
	if options.Factory == nil {
		return fmt.Errorf("factory is required")
	}

	httpServer, err := options.Factory.NewHTTPServer(config)
	if err != nil {
		return fmt.Errorf("create HTTP server: %w", err)
	}
	queueWorker, err := options.Factory.NewQueueWorker(config)
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
		return shutdownHTTP(httpServer, shutdownTimeout(config, options))
	case err := <-errs:
		cancel()
		if shutdownErr := shutdownHTTP(httpServer, shutdownTimeout(config, options)); shutdownErr != nil {
			return shutdownErr
		}
		if err != nil {
			return err
		}
		return nil
	}
}

func shutdownHTTP(httpServer HTTPServer, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	return nil
}

func shutdownTimeout(config config.Config, options Options) time.Duration {
	if options.ShutdownTimeout > 0 {
		return options.ShutdownTimeout
	}
	if config.ShutdownTimeout > 0 {
		return config.ShutdownTimeout
	}

	return 10 * time.Second
}

func (factory RuntimeFactory) NewHTTPServer(config config.Config) (HTTPServer, error) {
	dependencies := httpserver.Dependencies{
		MessageTable: config.MessageTable,
		ChatTable:    config.ChatIDTable,
		Store:        factory.Store,
		Enqueuer:     queue.Producer{QueueURL: config.EventQueueURL, Sender: factory.Sender},
		Messenger:    factory.Messenger,
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
