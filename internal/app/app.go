// Package app owns service lifecycle orchestration.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=app.go -destination=../mock/app_mock.go -package=mock -mock_names httpRunner=MockHTTPRunner,queueRunner=MockQueueWorker"

type httpRunner interface {
	Serve(net.Listener) error
	Shutdown(context.Context) error
	Close() error
}

type queueRunner interface {
	Run(context.Context) error
}

// HTTPServer defines one named HTTP server and its bind address.
type HTTPServer struct {
	name    string
	address string
	runner  httpRunner
}

// NewHTTPServer combines a server with the address that app will bind.
// For example, NewHTTPServer("metrics", ":9090", server) binds metrics on port 9090.
func NewHTTPServer(name string, address string, runner httpRunner) HTTPServer {
	return HTTPServer{name: name, address: address, runner: runner}
}

// runtimeApp wraps the configured application processes.
type runtimeApp struct {
	httpServer      HTTPServer
	metricsServer   HTTPServer
	queueWorker     queueRunner
	readiness       *atomic.Bool
	readinessDrain  time.Duration
	shutdownTimeout time.Duration
	listen          func(network string, address string) (net.Listener, error)
}

// Options configures application runtime behaviour.
type Options struct {
	Readiness       *atomic.Bool
	ReadinessDrain  time.Duration
	ShutdownTimeout time.Duration
}

// New constructs an application with the provided processes and options.
func New(httpServer HTTPServer, metricsServer HTTPServer, queueWorker queueRunner, options Options) *runtimeApp {
	return &runtimeApp{
		httpServer:      httpServer,
		metricsServer:   metricsServer,
		queueWorker:     queueWorker,
		readiness:       options.Readiness,
		readinessDrain:  readinessDrain(options),
		shutdownTimeout: shutdownTimeout(options),
		listen:          net.Listen,
	}
}

// Run binds and starts the configured processes.
func (app *runtimeApp) Run(ctx context.Context) error {
	err := app.validate()
	if err != nil {
		return err
	}
	app.readiness.Store(false)

	httpListener, err := app.listen("tcp", app.httpServer.address)
	if err != nil {
		return fmt.Errorf("listen HTTP server: %w", err)
	}
	metricsListener, err := app.listen("tcp", app.metricsServer.address)
	if err != nil {
		return errors.Join(fmt.Errorf("listen metrics server: %w", err), httpListener.Close())
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	group, groupCtx := errgroup.WithContext(runCtx)
	componentErrs := make(chan error, 3)
	var shutdownStarted atomic.Bool
	app.readiness.Store(true)
	startServer(group, componentErrs, app.httpServer, httpListener, app.readiness, &shutdownStarted)
	startServer(group, componentErrs, app.metricsServer, metricsListener, app.readiness, &shutdownStarted)
	group.Go(func() error {
		err := normaliseWorkerRunError(app.queueWorker.Run(groupCtx), groupCtx)
		if err != nil {
			app.readiness.Store(false)
		}
		logComponentStop("queue worker", err)
		componentErrs <- err
		return err
	})

	componentErr := waitForStop(ctx, componentErrs)
	app.readiness.Store(false)
	waitReadinessDrain(app.readinessDrain)
	shutdownStarted.Store(true)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), app.shutdownTimeout)
	defer shutdownCancel()
	shutdownErr, forced := shutdownServers(shutdownCtx, app.httpServer, app.metricsServer)
	waitErr := waitForGroup(shutdownCtx, group, app.httpServer, app.metricsServer, forced)
	if componentErr != nil {
		return errors.Join(componentErr, shutdownErr, waitErr)
	}

	return errors.Join(shutdownErr, waitErr)
}

func (app *runtimeApp) validate() error {
	if app == nil || app.httpServer.runner == nil {
		return fmt.Errorf("http server is required")
	}
	if app.metricsServer.runner == nil {
		return fmt.Errorf("metrics server is required")
	}
	if app.queueWorker == nil {
		return fmt.Errorf("queue worker is required")
	}
	if app.readiness == nil {
		return fmt.Errorf("readiness is required")
	}
	if app.listen == nil {
		return fmt.Errorf("listener is required")
	}

	return nil
}

func startServer(group *errgroup.Group, componentErrs chan<- error, server HTTPServer, listener net.Listener, readiness *atomic.Bool, shutdownStarted *atomic.Bool) {
	group.Go(func() error {
		err := normaliseHTTPServeError(server.runner.Serve(listener), shutdownStarted.Load(), server.name)
		if err != nil {
			readiness.Store(false)
		}
		logComponentStop(server.name+" server", err)
		componentErrs <- err
		return err
	})
}

func waitForStop(ctx context.Context, componentErrs <-chan error) error {
	select {
	case err := <-componentErrs:
		return err
	case <-ctx.Done():
		return nil
	}
}

func shutdownServers(ctx context.Context, servers ...HTTPServer) (error, bool) {
	errs := make(chan error, len(servers))
	for _, server := range servers {
		go func(server HTTPServer) {
			err := server.runner.Shutdown(ctx)
			if errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			if err != nil {
				errs <- fmt.Errorf("shutdown %s server: %w", server.name, err)
				return
			}
			errs <- nil
		}(server)
	}

	var shutdownErr error
	for range servers {
		select {
		case err := <-errs:
			shutdownErr = errors.Join(shutdownErr, err)
		case <-ctx.Done():
			return errors.Join(shutdownErr, ctx.Err(), closeServers(servers...)), true
		}
	}
	if shutdownErr != nil {
		return errors.Join(shutdownErr, closeServers(servers...)), true
	}

	return nil, false
}

func waitForGroup(ctx context.Context, group *errgroup.Group, httpServer HTTPServer, metricsServer HTTPServer, forced bool) error {
	done := make(chan error, 1)
	go func() {
		done <- group.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if forced {
			return ctx.Err()
		}
		return errors.Join(ctx.Err(), closeServers(httpServer, metricsServer))
	}
}

func closeServers(servers ...HTTPServer) error {
	var closeErr error
	for _, server := range servers {
		err := server.runner.Close()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			closeErr = errors.Join(closeErr, fmt.Errorf("close %s server: %w", server.name, err))
		}
	}

	return closeErr
}

func logComponentStop(component string, err error) {
	if err != nil {
		slog.Error(component+" stopped", "err", err)
		return
	}
	slog.Debug(component + " stopped")
}

// shutdownTimeout returns the configured shutdown timeout.
func shutdownTimeout(options Options) time.Duration {
	if options.ShutdownTimeout > 0 {
		return options.ShutdownTimeout
	}

	return 10 * time.Second
}

// readinessDrain returns the time for Kubernetes endpoint propagation.
// For example, an option of 5*time.Second waits five seconds before shutdown.
func readinessDrain(options Options) time.Duration {
	if options.ReadinessDrain > 0 {
		return options.ReadinessDrain
	}

	return 5 * time.Second
}

func waitReadinessDrain(delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	<-timer.C
}

// normaliseHTTPServeError keeps expected shutdown results separate from failures.
func normaliseHTTPServeError(err error, shutdownStarted bool, name string) error {
	if shutdownStarted && (err == nil || errors.Is(err, http.ErrServerClosed)) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("%s server stopped unexpectedly", name)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%s server stopped unexpectedly: %w", name, err)
	}

	return err
}

// normaliseWorkerRunError hides expected context-driven worker shutdown.
func normaliseWorkerRunError(err error, ctx context.Context) error {
	if ctx.Err() != nil && (err == nil || errors.Is(err, ctx.Err()) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("queue worker stopped unexpectedly")
	}

	return err
}
