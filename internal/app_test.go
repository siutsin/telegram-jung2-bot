package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
	"golang.org/x/sync/errgroup"

	appmock "github.com/siutsin/telegram-jung2-bot/internal/mock"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func TestRunMakesReadyAfterBothPortsBind(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readiness := &atomic.Bool{}
	httpServer, metricsServer, queueWorker := newMocks(t)
	started := make(chan string, 2)
	stopped := make(chan struct{})
	var stopOnce sync.Once
	expectBlockingServer(httpServer, "HTTP", started, stopped, &stopOnce)
	expectBlockingServer(metricsServer, "metrics", started, stopped, &stopOnce)
	queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(runCtx context.Context) error {
		<-runCtx.Done()
		return runCtx.Err()
	})
	application := newTestApp(httpServer, metricsServer, queueWorker, readiness)

	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	require.Eventually(t, readiness.Load, time.Second, time.Millisecond)
	got := make([]string, 0, 2)
	require.Eventually(t, func() bool {
		for len(got) < 2 {
			select {
			case name := <-started:
				got = append(got, name)
			default:
				return false
			}
		}
		return true
	}, time.Second, time.Millisecond)
	assert.ElementsMatch(t, []string{"HTTP", "metrics"}, got)

	cancel()
	require.NoError(t, <-done)
	assert.False(t, readiness.Load())
}

func TestRunStopsBothServersAfterComponentFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		{name: "HTTP"},
		{name: "metrics"},
		{name: "worker"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			readiness := &atomic.Bool{}
			httpServer, metricsServer, queueWorker := newMocks(t)
			stopped := make(chan struct{})
			var stopOnce sync.Once
			componentErr := errors.New(test.name + " boom")
			if test.name == "HTTP" {
				httpServer.EXPECT().Serve(gomock.Any()).Return(componentErr)
				expectShutdown(httpServer, stopped, &stopOnce)
			} else {
				expectBlockingServer(httpServer, "HTTP", nil, stopped, &stopOnce)
			}
			if test.name == "metrics" {
				metricsServer.EXPECT().Serve(gomock.Any()).Return(componentErr)
				expectShutdown(metricsServer, stopped, &stopOnce)
			} else {
				expectBlockingServer(metricsServer, "metrics", nil, stopped, &stopOnce)
			}
			if test.name == "worker" {
				queueWorker.EXPECT().Run(gomock.Any()).Return(componentErr)
			} else {
				queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(runCtx context.Context) error {
					<-runCtx.Done()
					return runCtx.Err()
				})
			}
			application := newTestApp(httpServer, metricsServer, queueWorker, readiness)

			err := application.Run(context.Background())
			require.ErrorIs(t, err, componentErr)
			assert.False(t, readiness.Load())
		})
	}
}

func TestRunHandlesConcurrentComponentFailures(t *testing.T) {
	t.Parallel()

	for attempt := range 10 {
		t.Run(fmt.Sprintf("attempt-%d", attempt), func(t *testing.T) {
			readiness := &atomic.Bool{}
			httpServer, metricsServer, queueWorker := newMocks(t)
			started := make(chan struct{}, 3)
			releaseFailures := make(chan struct{})

			httpServer.EXPECT().Serve(gomock.Any()).DoAndReturn(func(net.Listener) error {
				started <- struct{}{}
				<-releaseFailures
				return errors.New("HTTP failure")
			})
			metricsServer.EXPECT().Serve(gomock.Any()).DoAndReturn(func(net.Listener) error {
				started <- struct{}{}
				<-releaseFailures
				return errors.New("metrics failure")
			})
			queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(context.Context) error {
				started <- struct{}{}
				<-releaseFailures
				return errors.New("worker failure")
			})
			httpServer.EXPECT().Shutdown(gomock.Any()).Return(nil)
			metricsServer.EXPECT().Shutdown(gomock.Any()).Return(nil)

			application := newTestApp(httpServer, metricsServer, queueWorker, readiness)
			done := make(chan error, 1)
			go func() { done <- application.Run(context.Background()) }()
			for range 3 {
				<-started
			}
			go func() { close(releaseFailures) }()

			require.Eventually(t, func() bool { return !readiness.Load() }, time.Second, time.Millisecond)
			require.Error(t, <-done)
		})
	}
}

func TestRunTreatsUnexpectedServerStopsAsFailures(t *testing.T) {
	t.Parallel()

	for _, serverErr := range []error{nil, http.ErrServerClosed} {
		t.Run(errorName(serverErr), func(t *testing.T) {
			readiness := &atomic.Bool{}
			httpServer, metricsServer, queueWorker := newMocks(t)
			httpServer.EXPECT().Serve(gomock.Any()).Return(serverErr)
			stopped := make(chan struct{})
			var stopOnce sync.Once
			expectShutdown(httpServer, stopped, &stopOnce)
			expectBlockingServer(metricsServer, "metrics", nil, stopped, &stopOnce)
			queueWorker.EXPECT().Run(gomock.Any()).DoAndReturn(func(runCtx context.Context) error {
				<-runCtx.Done()
				return runCtx.Err()
			})

			err := newTestApp(httpServer, metricsServer, queueWorker, readiness).Run(context.Background())
			require.Error(t, err)
			assert.False(t, readiness.Load())
		})
	}
}

func TestRunReturnsShutdownErrorsAndAttemptsBothServers(t *testing.T) {
	t.Parallel()

	componentErr := errors.New("worker boom")
	shutdownErr := errors.New("shutdown boom")
	readiness := &atomic.Bool{}
	httpServer, metricsServer, queueWorker := newMocks(t)
	stopped := make(chan struct{})
	var stopOnce sync.Once
	httpServer.EXPECT().Serve(gomock.Any()).DoAndReturn(func(net.Listener) error {
		<-stopped
		return http.ErrServerClosed
	})
	httpServer.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(context.Context) error {
		stopOnce.Do(func() { close(stopped) })
		return shutdownErr
	})
	httpServer.EXPECT().Close().Return(nil)
	metricsServer.EXPECT().Serve(gomock.Any()).DoAndReturn(func(net.Listener) error {
		<-stopped
		return http.ErrServerClosed
	})
	metricsServer.EXPECT().Shutdown(gomock.Any()).Return(nil)
	metricsServer.EXPECT().Close().Return(nil)
	queueWorker.EXPECT().Run(gomock.Any()).Return(componentErr)

	err := newTestApp(httpServer, metricsServer, queueWorker, readiness).Run(context.Background())
	require.ErrorIs(t, err, componentErr)
	require.ErrorIs(t, err, shutdownErr)
}

func TestRunClosesFirstListenerWhenMetricsBindFails(t *testing.T) {
	t.Parallel()

	listenErr := errors.New("bind metrics")
	first, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	readiness := &atomic.Bool{}
	httpServer, metricsServer, queueWorker := newMocks(t)
	application := newTestApp(httpServer, metricsServer, queueWorker, readiness)
	calls := 0
	application.listen = func(network string, address string) (net.Listener, error) {
		calls++
		if calls == 1 {
			return first, nil
		}
		return nil, listenErr
	}

	err = application.Run(context.Background())
	require.ErrorIs(t, err, listenErr)
	_, acceptErr := first.Accept()
	require.ErrorIs(t, acceptErr, net.ErrClosed)
	assert.False(t, readiness.Load())
}

func TestRunReturnsHTTPBindError(t *testing.T) {
	t.Parallel()

	listenErr := errors.New("bind HTTP")
	readiness := &atomic.Bool{}
	httpServer, metricsServer, queueWorker := newMocks(t)
	application := newTestApp(httpServer, metricsServer, queueWorker, readiness)
	application.listen = func(string, string) (net.Listener, error) {
		return nil, listenErr
	}

	require.ErrorIs(t, application.Run(context.Background()), listenErr)
	assert.False(t, readiness.Load())
}

func TestValidateRequiresEveryProcess(t *testing.T) {
	t.Parallel()

	readiness := &atomic.Bool{}
	httpServer, metricsServer, queueWorker := newMocks(t)
	valid := newTestApp(httpServer, metricsServer, queueWorker, readiness)
	require.NoError(t, valid.validate())
	require.EqualError(t, (&runtimeApp{}).Run(context.Background()), "http server is required")
	require.EqualError(t, (&runtimeApp{}).validate(), "http server is required")
	require.EqualError(t, (&runtimeApp{httpServer: valid.httpServer}).validate(), "metrics server is required")
	require.EqualError(t, (&runtimeApp{httpServer: valid.httpServer, metricsServer: valid.metricsServer}).validate(), "queue worker is required")
	require.EqualError(t, (&runtimeApp{httpServer: valid.httpServer, metricsServer: valid.metricsServer, queueWorker: valid.queueWorker}).validate(), "readiness is required")
	require.EqualError(t, (&runtimeApp{httpServer: valid.httpServer, metricsServer: valid.metricsServer, queueWorker: valid.queueWorker, readiness: readiness}).validate(), "listener is required")
}

func TestNormaliseErrors(t *testing.T) {
	t.Parallel()

	assert.NoError(t, normaliseHTTPServeError(nil, true, "HTTP"))
	assert.NoError(t, normaliseHTTPServeError(http.ErrServerClosed, true, "HTTP"))
	require.EqualError(t, normaliseHTTPServeError(nil, false, "HTTP"), "HTTP server stopped unexpectedly")
	require.ErrorIs(t, normaliseHTTPServeError(http.ErrServerClosed, false, "HTTP"), http.ErrServerClosed)
	require.EqualError(t, normaliseWorkerRunError(nil, context.Background()), "queue worker stopped unexpectedly")
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	assert.NoError(t, normaliseWorkerRunError(context.Canceled, cancelled))
	assert.NoError(t, normaliseWorkerRunError(nil, cancelled))
}

func TestShutdownTimeoutUsesFallback(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 10*time.Second, shutdownTimeout(AppOptions{}))
	assert.Equal(t, time.Second, shutdownTimeout(AppOptions{ShutdownTimeout: time.Second}))
	assert.Equal(t, 5*time.Second, readinessDrain(AppOptions{}))
	assert.Equal(t, time.Second, readinessDrain(AppOptions{ReadinessDrain: time.Second}))
}

func TestWaitForStopAfterContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	componentErr := errors.New("component stopped")
	componentErrs := make(chan error, 1)
	componentErrs <- componentErr
	require.ErrorIs(t, waitForStop(context.Background(), componentErrs), componentErr)
	assert.NoError(t, waitForStop(ctx, make(chan error)))
}

func TestShutdownServersForcesCloseAfterTimeout(t *testing.T) {
	t.Parallel()

	httpServer, metricsServer, _ := newMocks(t)
	closeErr := errors.New("close HTTP server")
	started := make(chan struct{}, 2)
	expectShutdownDeadline(httpServer, started)
	expectShutdownDeadline(metricsServer, started)
	httpServer.EXPECT().Close().Return(closeErr)
	metricsServer.EXPECT().Close().Return(http.ErrServerClosed)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		err    error
		forced bool
	}
	done := make(chan result, 1)
	go func() {
		err, forced := shutdownServers(ctx,
			NewHTTPServer("HTTP", "", httpServer),
			NewHTTPServer("metrics", "", metricsServer),
		)
		done <- result{err: err, forced: forced}
	}()
	<-started
	<-started
	cancel()
	got := <-done
	err, forced := got.err, got.forced
	assert.True(t, forced)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, closeErr)
}

func TestShutdownServersIgnoresServerClosed(t *testing.T) {
	t.Parallel()

	httpServer, metricsServer, _ := newMocks(t)
	httpServer.EXPECT().Shutdown(gomock.Any()).Return(http.ErrServerClosed)
	metricsServer.EXPECT().Shutdown(gomock.Any()).Return(nil)
	err, forced := shutdownServers(context.Background(),
		NewHTTPServer("HTTP", "", httpServer),
		NewHTTPServer("metrics", "", metricsServer),
	)
	require.NoError(t, err)
	assert.False(t, forced)
}

func TestWaitForGroupTimeoutPaths(t *testing.T) {
	t.Parallel()

	t.Run("forced", func(t *testing.T) {
		httpServer, metricsServer, _ := newMocks(t)
		var group errgroup.Group
		release := make(chan struct{})
		group.Go(func() error {
			<-release
			return nil
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, waitForGroup(ctx, &group,
			NewHTTPServer("HTTP", "", httpServer),
			NewHTTPServer("metrics", "", metricsServer), true,
		), context.Canceled)
		close(release)
	})

	t.Run("close", func(t *testing.T) {
		httpServer, metricsServer, _ := newMocks(t)
		httpServer.EXPECT().Close().Return(nil)
		metricsServer.EXPECT().Close().Return(nil)
		var group errgroup.Group
		release := make(chan struct{})
		group.Go(func() error {
			<-release
			return nil
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, waitForGroup(ctx, &group,
			NewHTTPServer("HTTP", "", httpServer),
			NewHTTPServer("metrics", "", metricsServer), false,
		), context.Canceled)
		close(release)
	})

	t.Run("group error", func(t *testing.T) {
		groupErr := errors.New("group stopped")
		var group errgroup.Group
		group.Go(func() error { return groupErr })
		require.ErrorIs(t, waitForGroup(context.Background(), &group,
			HTTPServer{}, HTTPServer{}, false), groupErr)
	})
}

func newMocks(t *testing.T) (*appmock.MockHTTPRunner, *appmock.MockHTTPRunner, *appmock.MockQueueWorker) {
	t.Helper()
	controller := gomock.NewController(t)
	return appmock.NewMockHTTPRunner(controller), appmock.NewMockHTTPRunner(controller), appmock.NewMockQueueWorker(controller)
}

func newTestApp(httpServer httpRunner, metricsServer httpRunner, worker queueRunner, readiness *atomic.Bool) *runtimeApp {
	return NewApp(
		NewHTTPServer("HTTP", "127.0.0.1:0", httpServer),
		NewHTTPServer("metrics", "127.0.0.1:0", metricsServer),
		worker,
		AppOptions{Readiness: readiness, ReadinessDrain: time.Nanosecond, ShutdownTimeout: time.Second},
	)
}

func expectBlockingServer(server *appmock.MockHTTPRunner, name string, started chan<- string, stopped chan struct{}, stopOnce *sync.Once) {
	server.EXPECT().Serve(gomock.Any()).DoAndReturn(func(net.Listener) error {
		if started != nil {
			started <- name
		}
		<-stopped
		return http.ErrServerClosed
	})
	expectShutdown(server, stopped, stopOnce)
}

func expectShutdown(server *appmock.MockHTTPRunner, stopped chan struct{}, stopOnce *sync.Once) {
	server.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(context.Context) error {
		stopOnce.Do(func() { close(stopped) })
		return nil
	})
}

func expectShutdownDeadline(server *appmock.MockHTTPRunner, started chan<- struct{}) {
	server.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		started <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	})
}

func errorName(err error) string {
	if err == nil {
		return "nil"
	}
	return "server closed"
}
