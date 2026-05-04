package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	gomock "go.uber.org/mock/gomock"

	appmock "github.com/siutsin/telegram-jung2-bot/internal/mock"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func newMocks(t *testing.T) (*appmock.MockHTTPRunner, *appmock.MockQueueWorker) {
	t.Helper()

	controller := gomock.NewController(t)

	return appmock.NewMockHTTPRunner(controller), appmock.NewMockQueueWorker(controller)
}

func runWithCancel(t *testing.T, application *runtimeApp, ctx context.Context, cancel context.CancelFunc, started <-chan struct{}) error {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- application.Run(ctx)
	}()

	<-started
	cancel()

	return <-done
}

func runAfterStarts(t *testing.T, application *runtimeApp, ctx context.Context, release func(), started <-chan struct{}, count int) error {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- application.Run(ctx)
	}()

	for range count {
		<-started
	}
	release()

	return <-done
}
