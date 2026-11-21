package httpserver

import (
	"context"
	"errors"
	"fmt"
	"testing"

	bot "github.com/siutsin/telegram-jung2-bot/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

// TestHandleWebhookSavesAndEnqueuesCommand preserves independent save and command enqueue work.
func TestHandleWebhookSavesAndEnqueuesCommand(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	dependencies.Metrics = bot.NewMetrics(nil)
	mocks.expectSaveWebhookState()
	mocks.expectEnqueue(nil)
	got := handleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"supergroup"},"from":{"id":456},"text":"/topTen","entities":[{"type":"bot_command"}]}}`), dependencies)

	assert.Equal(t, response{statusCode: 200}, got)
	require.Len(t, mocks.messageActions, 1)
	assert.Equal(t, queue.ActionSaveMessage, mocks.messageActions[0].Name)
	assert.Equal(t, "123", mocks.messageActions[0].Attributes["chatId"])
	assert.Equal(t, "123:0", mocks.messageActions[0].MessageDeduplicationID)
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

// TestHandleWebhookSavesMessagesWithoutEnqueue keeps every group message countable without a command.
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
			assert.Len(t, mocks.messageActions, 1)
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
			name:    "enqueue message save",
			payload: groupMessage,
			setup: func(mocks *httpserverMocks) {
				mocks.expectMessageEnqueue(errors.New("boom"))
			},
			want: response{statusCode: 500, message: "enqueue message save"},
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

// TestHandleWebhookCarriesTelegramMessageIdentity preserves FIFO deduplication and DynamoDB row identity.
func TestHandleWebhookCarriesTelegramMessageIdentity(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	got := handleWebhook(context.Background(), []byte(`{"update_id":789,"message":{"message_id":456,"date":1777723200,"chat":{"id":123,"title":"Group","type":"group"},"text":"hello"}}`), dependencies)

	assert.Equal(t, 200, got.statusCode)
	assert.Equal(t, "456", mocks.messageActions[0].Attributes["messageId"])
	assert.Equal(t, "1777723200", mocks.messageActions[0].Attributes["date"])
	assert.Equal(t, "123:789", mocks.messageActions[0].MessageDeduplicationID)
}

// TestHandleWebhookSaveActionFitsSQSAttributeLimit preserves the SQS hard
// cap of ten message attributes per message. The queue producer adds one
// more attribute ("enqueuedAt") before sending, so a fully populated
// message-save action must carry nine or fewer of its own.
func TestHandleWebhookSaveActionFitsSQSAttributeLimit(t *testing.T) {
	t.Parallel()

	const sqsMaxMessageAttributes = 10
	mocks, dependencies := newMockDependencies(t)
	mocks.expectSaveWebhookState()
	got := handleWebhook(context.Background(), []byte(`{"update_id":789,"message":{"message_id":456,"date":1777723200,"chat":{"id":123,"title":"Group","type":"group"},"from":{"id":456,"username":"username","first_name":"first","last_name":"last"},"text":"hello"}}`), dependencies)

	assert.Equal(t, 200, got.statusCode)
	require.Len(t, mocks.messageActions, 1)
	assert.LessOrEqual(t, len(mocks.messageActions[0].Attributes), sqsMaxMessageAttributes-1)
}

func TestEnqueueWebhookCommandIgnoresUnsupportedCommand(t *testing.T) {
	t.Parallel()

	_, dependencies := newMockDependencies(t)
	result, ok := enqueueWebhookCommand(
		context.Background(),
		bot.TelegramMessage{Chat: bot.Chat{ID: 123, Type: "group"}},
		bot.Command{Name: "unsupported"},
		dependencies,
	)

	assert.True(t, ok)
	assert.Equal(t, response{}, result)
}

// myChatMemberPayload builds a my_chat_member update for chat 123.
// For example, ("supergroup", "member", "left") is the bot leaving a supergroup.
func myChatMemberPayload(chatType, oldStatus, newStatus string) string {
	return fmt.Sprintf(`{"update_id":1,"my_chat_member":{"chat":{"id":123,"title":"Group","type":%q},"from":{"id":999},"date":1777723200,"old_chat_member":{"user":{"id":999},"status":%q},"new_chat_member":{"user":{"id":999},"status":%q}}}`, chatType, oldStatus, newStatus)
}

// TestHandleWebhookDeletesChatOnBotRemoval covers the left and kicked removals.
func TestHandleWebhookDeletesChatOnBotRemoval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		newStatus string
	}{
		{
			name:      "left",
			newStatus: "left",
		},
		{
			name:      "kicked",
			newStatus: "kicked",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mocks, dependencies := newMockDependencies(t)
			mocks.expectDeleteChat(nil)
			got := handleWebhook(context.Background(), []byte(myChatMemberPayload("supergroup", "member", tc.newStatus)), dependencies)

			assert.Equal(t, response{statusCode: 200}, got)
			assert.Equal(t, []int64{123}, mocks.deletedChatIDs)
		})
	}
}

// TestHandleWebhookIgnoresMyChatMemberWithoutRemoval covers every my_chat_member
// update that must not delete the chat row.
func TestHandleWebhookIgnoresMyChatMemberWithoutRemoval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		chatType  string
		oldStatus string
		newStatus string
	}{
		{
			name:      "member to administrator",
			chatType:  "supergroup",
			oldStatus: "member",
			newStatus: "administrator",
		},
		{
			name:      "restricted",
			chatType:  "group",
			oldStatus: "member",
			newStatus: "restricted",
		},
		{
			name:      "non-group left",
			chatType:  "private",
			oldStatus: "member",
			newStatus: "left",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, dependencies := newMockDependencies(t)
			got := handleWebhook(context.Background(), []byte(myChatMemberPayload(tc.chatType, tc.oldStatus, tc.newStatus)), dependencies)

			assert.Equal(t, response{statusCode: 204, message: "edited_message or non-group"}, got)
		})
	}
}

// TestHandleWebhookReturns200WhenChatDeleteFails keeps a failed deletion from
// forcing a Telegram retry.
func TestHandleWebhookReturns200WhenChatDeleteFails(t *testing.T) {
	t.Parallel()

	mocks, dependencies := newMockDependencies(t)
	mocks.expectDeleteChat(errors.New("boom"))
	got := handleWebhook(context.Background(), []byte(myChatMemberPayload("group", "member", "left")), dependencies)

	assert.Equal(t, response{statusCode: 200}, got)
	assert.Equal(t, []int64{123}, mocks.deletedChatIDs)
}
