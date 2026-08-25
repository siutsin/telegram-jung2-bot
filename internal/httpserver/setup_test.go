package httpserver

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	gomock "go.uber.org/mock/gomock"

	httpservermock "github.com/siutsin/telegram-jung2-bot/internal/mock/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

type httpserverMocks struct {
	enqueuer        *httpservermock.MockEnqueuer
	messageEnqueuer *httpservermock.MockEnqueuer
	messenger       *httpservermock.MockMessenger
	scaleUpper      *httpservermock.MockScaleUpper

	actions        []queue.Action
	messageActions []queue.Action
	sentMessages   []string
}

func newMockDependencies(t *testing.T) (*httpserverMocks, Dependencies) {
	t.Helper()

	controller := gomock.NewController(t)
	mocks := &httpserverMocks{
		enqueuer:        httpservermock.NewMockEnqueuer(controller),
		messageEnqueuer: httpservermock.NewMockEnqueuer(controller),
		messenger:       httpservermock.NewMockMessenger(controller),
		scaleUpper:      httpservermock.NewMockScaleUpper(controller),
	}

	readiness := &atomic.Bool{}
	readiness.Store(true)

	return mocks, Dependencies{
		Enqueuer:        mocks.enqueuer,
		MessageEnqueuer: mocks.messageEnqueuer,
		Messenger:       mocks.messenger,
		Now: func() time.Time {
			return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		},
		Readiness: readiness,
	}
}

func (mocks *httpserverMocks) expectSaveWebhookState() {
	mocks.expectMessageEnqueue(nil)
}

// expectMessageEnqueue records asynchronous message-save work for webhook tests.
func (mocks *httpserverMocks) expectMessageEnqueue(err error) {
	mocks.messageEnqueuer.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, action queue.Action) error {
			mocks.messageActions = append(mocks.messageActions, action)
			return err
		})
}

func (mocks *httpserverMocks) expectEnqueue(err error) {
	mocks.enqueuer.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, action queue.Action) error {
			mocks.actions = append(mocks.actions, action)
			return err
		})
}

func (mocks *httpserverMocks) expectSendMessage(err error) {
	mocks.messenger.EXPECT().
		SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, chatID int64, text string) error {
			mocks.sentMessages = append(mocks.sentMessages, text)
			return err
		})
}

func (mocks *httpserverMocks) expectScaleUp(err error) {
	mocks.scaleUpper.EXPECT().ScaleUp(gomock.Any()).Return(err)
}
