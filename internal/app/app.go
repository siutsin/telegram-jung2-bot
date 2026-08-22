// Package app owns service lifecycle orchestration.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
)

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=app.go -destination=../mock/app_mock.go -package=mock -mock_names httpRunner=MockHTTPRunner,queueRunner=MockQueueWorker"

type httpRunner interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type queueRunner interface {
	Run(ctx context.Context) error
}

// runtimeApp wraps the configured application processes.
type runtimeApp struct {
	httpServer      httpRunner
	queueWorker     queueRunner
	shutdownTimeout time.Duration
}

// Options configures application runtime behaviour.
type Options struct {
	ShutdownTimeout time.Duration
}

// New constructs an application with the provided processes and options.
func New(httpServer httpRunner, queueWorker queueRunner, options Options) *runtimeApp {
	return &runtimeApp{
		httpServer:      httpServer,
		queueWorker:     queueWorker,
		shutdownTimeout: shutdownTimeout(options),
	}
}

// Run starts the configured application.
func (app *runtimeApp) Run(ctx context.Context) error {
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
		err := normaliseHTTPServeError(app.httpServer.ListenAndServe())
		if err != nil {
			slog.Error("http server stopped", "err", err)
		} else {
			slog.Debug("http server stopped")
		}
		componentErrs <- err
		return err
	})
	group.Go(func() error {
		err := normaliseWorkerRunError(app.queueWorker.Run(groupCtx), groupCtx)
		if err != nil {
			slog.Error("queue worker stopped", "err", err)
		} else {
			slog.Debug("queue worker stopped")
		}
		componentErrs <- err
		return err
	})

	var componentErr error

	select {
	case <-ctx.Done():
		slog.Debug("application context cancelled")
		cancel()
	case componentErr = <-componentErrs:
		if componentErr != nil {
			slog.Error("application component failed", "err", componentErr)
		}
		cancel()
	}

	shutdownErr := shutdownHTTP(app.httpServer, app.shutdownTimeout)
	waitErr := group.Wait()
	if shutdownErr != nil {
		slog.Error("application shutdown failed", "err", shutdownErr)
		return shutdownErr
	}
	if componentErr != nil {
		return componentErr
	}

	return waitErr
}

// shutdownHTTP stops the HTTP server with a timeout.
func shutdownHTTP(httpServer httpRunner, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	slog.Debug("shutting down HTTP server", "timeout", timeout)
	err := httpServer.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	slog.Debug("HTTP server shutdown complete")
	return nil
}

// shutdownTimeout returns the configured shutdown timeout.
func shutdownTimeout(options Options) time.Duration {
	if options.ShutdownTimeout > 0 {
		return options.ShutdownTimeout
	}

	return 10 * time.Second
}

// normaliseHTTPServeError hides the expected server-closed stop result.
func normaliseHTTPServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

// normaliseWorkerRunError hides expected context-driven worker shutdown.
func normaliseWorkerRunError(err error, ctx context.Context) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil && (errors.Is(err, ctx.Err()) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return nil
	}

	return err
}
