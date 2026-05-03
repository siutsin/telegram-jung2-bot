// Package app owns service lifecycle orchestration.
package app

import (
	"context"
	"fmt"
	"time"
)

type HTTPRunner interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type QueueWorker interface {
	Run(ctx context.Context) error
}

// App wraps the configured application processes.
type App struct {
	httpServer      HTTPRunner
	queueWorker     QueueWorker
	shutdownTimeout time.Duration
}

// Options configures application runtime behaviour.
type Options struct {
	ShutdownTimeout time.Duration
}

// New constructs an application with the provided processes and options.
func New(httpServer HTTPRunner, queueWorker QueueWorker, options Options) *App {
	return &App{
		httpServer:      httpServer,
		queueWorker:     queueWorker,
		shutdownTimeout: shutdownTimeout(options),
	}
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
		shutdownErr := shutdownHTTP(app.httpServer, app.shutdownTimeout)
		if shutdownErr != nil {
			return shutdownErr
		}
		if err != nil {
			return err
		}
		return nil
	}
}

// shutdownHTTP stops the HTTP server with a timeout.
func shutdownHTTP(httpServer HTTPRunner, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := httpServer.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	return nil
}

// shutdownTimeout returns the configured shutdown timeout.
func shutdownTimeout(options Options) time.Duration {
	if options.ShutdownTimeout > 0 {
		return options.ShutdownTimeout
	}

	return 10 * time.Second
}
