package httpserver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/command"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

func TestHandleWebhookSavesAndEnqueuesCommand(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	mocks.expectEnqueue(nil)
	got := handleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"supergroup"},"from":{"id":456},"text":"/topTen","entities":[{"type":"bot_command"}]}}`), dependencies)

	assert.Equal(t, response{statusCode: 200}, got)
	require.Len(t, mocks.savedMessages, 1)
	assert.Equal(t, int64(123), mocks.savedMessages[0].ChatID)
	assert.Equal(t, "2026-05-02T20:00:00+08:00", message.FormatDateCreated(mocks.savedMessages[0].DateCreated))
	require.Len(t, mocks.savedChats, 1)
	assert.Equal(t, int64(123), mocks.savedChats[0].ChatID)
	require.Len(t, mocks.actions, 1)
	assert.Equal(t, queue.ActionTopTen, mocks.actions[0].Name)
	assert.Equal(t, "123", mocks.actions[0].Attributes["chatId"])
}

func TestHandleWebhookEnqueuesMultipleCommandsInContractOrder(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	mocks.expectEnqueue(nil)
	mocks.expectEnqueue(nil)
	mocks.expectEnqueue(nil)
	got := handleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/allJung /topTen /jungHelp","entities":[{"type":"bot_command"}]}}`), dependencies)

	assert.Equal(t, response{statusCode: 200}, got)
	require.Len(t, mocks.actions, 3)
	assert.Equal(t, queue.ActionJungHelp, mocks.actions[0].Name)
	assert.Equal(t, queue.ActionTopTen, mocks.actions[1].Name)
	assert.Equal(t, queue.ActionAllJung, mocks.actions[2].Name)
}

func TestHandleWebhookInvalidSetOffDoesNotBlockOtherCommands(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	mocks.expectSendMessage(nil)
	mocks.expectEnqueue(nil)
	got := handleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"from":{"id":456},"text":"/setOffFromWorkTimeUTC bad /topTen","entities":[{"type":"bot_command"}]}}`), dependencies)

	assert.Equal(t, response{statusCode: 200}, got)
	require.Len(t, mocks.sentMessages, 1)
	require.Len(t, mocks.actions, 1)
	assert.Equal(t, queue.ActionTopTen, mocks.actions[0].Name)
}

func TestHandleWebhookReturnsParseResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		want    response
	}{
		{
			name:    "unsupported update",
			payload: `{"edited_message":{"text":"ignored"}}`,
			want:    response{statusCode: 204, message: "edited_message or non-group"},
		},
		{
			name:    "non group",
			payload: `{"message":{"chat":{"id":123,"type":"private"},"text":"hi"}}`,
			want:    response{statusCode: 204, message: "edited_message or non-group"},
		},
		{
			name:    "invalid JSON",
			payload: `{bad json`,
			want:    response{statusCode: 500, message: "decode Telegram update"},
		},
		{
			name:    "malformed chat type",
			payload: `{"message":{"chat":{"id":123},"text":"hi"}}`,
			want:    response{statusCode: 500, message: "decode Telegram update"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, dependencies := newMockDependencies(t)
			got := handleWebhook(context.Background(), []byte(tc.payload), dependencies)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHandleWebhookSavesMessagesWithoutEnqueue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "plain message",
			payload: `{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"hello"}}`,
		},
		{
			name:    "slash text without bot command entity",
			payload: `{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/topTen"}}`,
		},
		{
			name:    "bot command is not first entity",
			payload: `{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/topTen","entities":[{"type":"mention"},{"type":"bot_command"}]}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mocks, dependencies := newMockDependencies(t)
			mocks.expectSaveWebhookState()
			got := handleWebhook(context.Background(), []byte(tc.payload), dependencies)

			assert.Equal(t, response{statusCode: 200}, got)
			assert.Len(t, mocks.savedMessages, 1)
			assert.Empty(t, mocks.actions)
		})
	}
}

func TestHandleWebhookSendsInvalidSetOffReply(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	mocks.expectSendMessage(nil)
	got := handleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"from":{"id":456},"text":"/setOffFromWorkTimeUTC bad","entities":[{"type":"bot_command"}]}}`), dependencies)

	assert.Equal(t, response{statusCode: 200}, got)
	require.Len(t, mocks.sentMessages, 1)
	assert.Contains(t, mocks.sentMessages[0], "Error: Invalid format for setOffFromWorkTimeUTC")
}

func TestHandleWebhookReturnsReplyErrorWhenMessengerIsMissing(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	dependencies.Messenger = nil
	got := handleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/setOffFromWorkTimeUTC bad","entities":[{"type":"bot_command"}]}}`), dependencies)

	assert.Equal(t, response{statusCode: 500, message: "reply invalid command"}, got)
}

func TestHandleWebhookReturnsDependencyErrors(t *testing.T) {
	t.Parallel()

	groupMessage := `{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"hello"}}`
	commandMessage := `{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/jungHelp","entities":[{"type":"bot_command"}]}}`
	tests := []struct {
		name    string
		payload string
		setup   func(*httpserverMocks)
		want    response
	}{
		{
			name:    "save message",
			payload: groupMessage,
			setup: func(mocks *httpserverMocks) {
				mocks.expectSaveMessage(errors.New("boom"))
			},
			want: response{statusCode: 500, message: "save message"},
		},
		{
			name:    "save chat",
			payload: groupMessage,
			setup: func(mocks *httpserverMocks) {
				mocks.expectSaveMessage(nil)
				mocks.expectSaveChat(errors.New("boom"))
			},
			want: response{statusCode: 500, message: "save chat"},
		},
		{
			name:    "enqueue",
			payload: commandMessage,
			setup: func(mocks *httpserverMocks) {
				mocks.expectSaveWebhookState()
				mocks.expectEnqueue(errors.New("boom"))
			},
			want: response{statusCode: 500, message: "enqueue command"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mocks, dependencies := newMockDependencies(t)
			tc.setup(mocks)
			got := handleWebhook(context.Background(), []byte(tc.payload), dependencies)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHandleWebhookDefaultsTime(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	dependencies.Now = nil
	got := handleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"hello"}}`), dependencies)

	assert.Equal(t, 200, got.statusCode)
	assert.False(t, mocks.savedMessages[0].DateCreated.IsZero())
}

func TestEnqueueWebhookCommandIgnoresUnsupportedCommand(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	result, ok := enqueueWebhookCommand(
		context.Background(),
		telegram.Message{Chat: telegram.Chat{ID: 123, Type: "group"}},
		command.Command{Name: "unsupported"},
		dependencies,
	)

	assert.True(t, ok)
	assert.Equal(t, response{}, result)
}
