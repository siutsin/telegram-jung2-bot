// Package app owns service lifecycle orchestration.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
)

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=app.go -destination=../mock/app_mock.go -package=mock"

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
	group, groupCtx := errgroup.WithContext(runCtx)
	componentErrs := make(chan error, 2)
	group.Go(func() error {
		err := normalizeHTTPServeError(app.httpServer.ListenAndServe())
		componentErrs <- err
		return err
	})
	group.Go(func() error {
		err := normalizeWorkerRunError(app.queueWorker.Run(groupCtx), groupCtx)
		componentErrs <- err
		return err
	})

	var componentErr error

	select {
	case <-ctx.Done():
		cancel()
	case componentErr = <-componentErrs:
		cancel()
	}

	shutdownErr := shutdownHTTP(app.httpServer, app.shutdownTimeout)
	waitErr := group.Wait()
	if shutdownErr != nil {
		return shutdownErr
	}
	if componentErr != nil {
		return componentErr
	}

	return waitErr
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

// normalizeHTTPServeError hides the expected server-closed stop result.
func normalizeHTTPServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

// normalizeWorkerRunError hides expected context-driven worker shutdown.
func normalizeWorkerRunError(err error, ctx context.Context) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil && (errors.Is(err, ctx.Err()) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return nil
	}

	return err
}
