package httpserver

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	gomock "go.uber.org/mock/gomock"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	httpservermock "github.com/siutsin/telegram-jung2-bot/internal/mock/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

type httpserverMocks struct {
	messages   *httpservermock.MockMessageSaver
	chats      *httpservermock.MockChatSaver
	enqueuer   *httpservermock.MockEnqueuer
	messenger  *httpservermock.MockMessenger
	scaleUpper *httpservermock.MockScaleUpper

	savedMessages []message.Message
	savedChats    []chat.ChatSetting
	actions       []queue.Action
	sentMessages  []string
}

func newMockDependencies(t *testing.T) (*httpserverMocks, Dependencies) {
	t.Helper()

	controller := gomock.NewController(t)
	mocks := &httpserverMocks{
		messages:   httpservermock.NewMockMessageSaver(controller),
		chats:      httpservermock.NewMockChatSaver(controller),
		enqueuer:   httpservermock.NewMockEnqueuer(controller),
		messenger:  httpservermock.NewMockMessenger(controller),
		scaleUpper: httpservermock.NewMockScaleUpper(controller),
	}

	return mocks, Dependencies{
		Messages:  mocks.messages,
		Chats:     mocks.chats,
		Enqueuer:  mocks.enqueuer,
		Messenger: mocks.messenger,
		Now: func() time.Time {
			return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		},
	}
}

func (mocks *httpserverMocks) expectSaveMessage(err error) {
	mocks.messages.EXPECT().
		Save(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, tableName string, row message.Message) error {
			mocks.savedMessages = append(mocks.savedMessages, row)
			return err
		})
}

func (mocks *httpserverMocks) expectSaveChat(err error) {
	mocks.chats.EXPECT().
		Save(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, tableName string, settings chat.ChatSetting) error {
			mocks.savedChats = append(mocks.savedChats, settings)
			return err
		})
}

func (mocks *httpserverMocks) expectSaveWebhookState() {
	mocks.expectSaveMessage(nil)
	mocks.expectSaveChat(nil)
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
