package bot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

// TestMessageSaveWorkerSavesAndDeletesBatch preserves FIFO save and batched delete behaviour.
func TestMessageSaveWorkerSavesAndDeletesBatch(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	controller := gomock.NewController(t)
	receiver := NewMockQueueReceiver(controller)
	deleter := NewMockQueueBatchDeleter(controller)
	messages := NewMockStoredMessageSaver(controller)
	chats := NewMockStoredChatSaver(controller)
	response := queue.ReceiveMessageResponse{Messages: []queue.RawMessage{
		saveRawMessage(t, "one", "10", "1760000000"),
		saveRawMessage(t, "two", "11", "1760000001"),
	}}
	receiver.EXPECT().ReceiveMessage(gomock.Any(), gomock.Any()).Return(response, nil)
	var messageRows []StoredMessage
	messages.EXPECT().Save(gomock.Any(), "messages", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, row StoredMessage) error {
		messageRows = append(messageRows, row)
		return nil
	}).Times(2)
	var chatRows []ChatSetting
	chats.EXPECT().Save(gomock.Any(), "chats", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, row ChatSetting) error {
		chatRows = append(chatRows, row)
		return nil
	}).Times(2)
	var deleteRequests []queue.DeleteMessageBatchRequest
	deleter.EXPECT().DeleteBatch(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, request queue.DeleteMessageBatchRequest) error {
		deleteRequests = append(deleteRequests, request)
		cancel()
		return nil
	})
	worker, err := NewMessageSaveWorker(
		"fifo-url",
		receiver,
		deleter,
		messages,
		"messages",
		chats,
		"chats",
		func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
		NewMetrics(nil),
	)
	require.NoError(t, err)

	require.NoError(t, worker.Run(ctx))
	require.Len(t, messageRows, 2)
	assert.Equal(t, int64(10), messageRows[0].MessageID)
	assert.Equal(t, "2025-10-09T16:53:20+08:00", FormatDateCreated(messageRows[0].DateCreated))
	assert.Len(t, chatRows, 2)
	assert.Equal(t, []string{"one", "two"}, deleteRequests[0].ReceiptHandles)
}

// TestMessageSaveWorkerLeavesFailedMessage keeps DynamoDB errors retryable in SQS.
func TestMessageSaveWorkerLeavesFailedMessage(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	controller := gomock.NewController(t)
	receiver := NewMockQueueReceiver(controller)
	deleter := NewMockQueueBatchDeleter(controller)
	messages := NewMockStoredMessageSaver(controller)
	chats := NewMockStoredChatSaver(controller)
	receiver.EXPECT().ReceiveMessage(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error) {
		cancel()
		return queue.ReceiveMessageResponse{Messages: []queue.RawMessage{saveRawMessage(t, "one", "10", "1760000000")}}, nil
	})
	messages.EXPECT().Save(gomock.Any(), "messages", gomock.Any()).Return(errors.New("down"))
	worker, err := NewMessageSaveWorker(
		"fifo-url",
		receiver,
		deleter,
		messages,
		"messages",
		chats,
		"chats",
		time.Now,
		nil,
	)
	require.NoError(t, err)

	require.NoError(t, worker.Run(ctx))
}

// TestNewMessageSaveWorkerRequiresDependencies prevents a worker from discarding queued messages.
func TestNewMessageSaveWorkerRequiresDependencies(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	receiver := NewMockQueueReceiver(controller)
	deleter := NewMockQueueBatchDeleter(controller)
	messages := NewMockStoredMessageSaver(controller)
	chats := NewMockStoredChatSaver(controller)
	for _, test := range []struct {
		name string
		err  string
		call func() error
	}{
		{"receiver", "queue receiver is required", func() error {
			_, err := NewMessageSaveWorker("", nil, deleter, messages, "", chats, "", nil, nil)
			return err
		}},
		{"deleter", "queue batch deleter is required", func() error {
			_, err := NewMessageSaveWorker("", receiver, nil, messages, "", chats, "", nil, nil)
			return err
		}},
		{"messages", "message store is required", func() error {
			_, err := NewMessageSaveWorker("", receiver, deleter, nil, "", chats, "", nil, nil)
			return err
		}},
		{"chats", "chat store is required", func() error {
			_, err := NewMessageSaveWorker("", receiver, deleter, messages, "", nil, "", nil, nil)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) { require.EqualError(t, test.call(), test.err) })
	}
}

// TestTelegramMessageFromActionRejectsInvalidInput prevents malformed queue records from reaching DynamoDB.
func TestTelegramMessageFromActionRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := telegramMessageFromAction(queue.Action{Name: queue.ActionSaveMessage, Attributes: map[string]string{"chatId": "1", "messageId": "2", "date": "0"}})
	require.EqualError(t, err, "invalid date for saveMessage")
	_, err = telegramMessageFromAction(queue.Action{Name: queue.ActionSaveMessage, Attributes: map[string]string{"chatId": "bad"}})
	require.ErrorContains(t, err, "invalid chatId")
}

// TestMessageSaveWorkerReturnsDeleteError stops after a batch-delete failure.
func TestMessageSaveWorkerReturnsDeleteError(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	receiver := NewMockQueueReceiver(controller)
	deleter := NewMockQueueBatchDeleter(controller)
	messages := NewMockStoredMessageSaver(controller)
	chats := NewMockStoredChatSaver(controller)
	receiver.EXPECT().ReceiveMessage(gomock.Any(), gomock.Any()).Return(queue.ReceiveMessageResponse{Messages: []queue.RawMessage{saveRawMessage(t, "one", "1", "1760000000")}}, nil)
	messages.EXPECT().Save(gomock.Any(), "messages", gomock.Any()).Return(nil)
	chats.EXPECT().Save(gomock.Any(), "chats", gomock.Any()).Return(nil)
	deleter.EXPECT().DeleteBatch(gomock.Any(), gomock.Any()).Return(errors.New("delete down"))
	worker, err := NewMessageSaveWorker("fifo", receiver, deleter, messages, "messages", chats, "chats", time.Now, nil)
	require.NoError(t, err)
	require.EqualError(t, worker.Run(context.Background()), "delete down")
}

// TestMessageSaveWorkerReturnsReceiveError stops when SQS cannot supply a batch.
func TestMessageSaveWorkerReturnsReceiveError(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	receiver := NewMockQueueReceiver(controller)
	receiver.EXPECT().ReceiveMessage(gomock.Any(), gomock.Any()).Return(queue.ReceiveMessageResponse{}, errors.New("receive down"))
	worker, err := NewMessageSaveWorker("fifo", receiver, NewMockQueueBatchDeleter(controller), NewMockStoredMessageSaver(controller), "messages", NewMockStoredChatSaver(controller), "chats", nil, nil)
	require.NoError(t, err)

	require.EqualError(t, worker.Run(context.Background()), "receive down")
}

// TestMessageSaveWorkerLeavesChatSaveFailures retries message metadata with the next SQS delivery.
func TestMessageSaveWorkerLeavesChatSaveFailures(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	controller := gomock.NewController(t)
	receiver := NewMockQueueReceiver(controller)
	messages := NewMockStoredMessageSaver(controller)
	chats := NewMockStoredChatSaver(controller)
	receiver.EXPECT().ReceiveMessage(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error) {
		cancel()
		return queue.ReceiveMessageResponse{Messages: []queue.RawMessage{saveRawMessage(t, "one", "1", "1760000000")}}, nil
	})
	messages.EXPECT().Save(gomock.Any(), "messages", gomock.Any()).Return(nil)
	chats.EXPECT().Save(gomock.Any(), "chats", gomock.Any()).Return(errors.New("chat down"))
	worker, err := NewMessageSaveWorker("fifo", receiver, NewMockQueueBatchDeleter(controller), messages, "messages", chats, "chats", nil, nil)
	require.NoError(t, err)

	require.NoError(t, worker.Run(ctx))
}

// TestTelegramMessageFromActionRejectsMissingAndInvalidFields keeps malformed queue data out of DynamoDB.
func TestTelegramMessageFromActionRejectsMissingAndInvalidFields(t *testing.T) {
	t.Parallel()

	for _, attributes := range []map[string]string{
		{"messageId": "2", "date": "1760000000"},
		{"chatId": "1", "date": "1760000000"},
		{"chatId": "1", "messageId": "2"},
		{"chatId": "1", "messageId": "2", "date": "1760000000", "userId": "bad"},
	} {
		_, err := telegramMessageFromAction(queue.Action{Name: queue.ActionSaveMessage, Attributes: attributes})
		require.Error(t, err)
	}
}

// TestMessageSaveWorkerRejectsUnknownActions keeps a wrong queue consumer from writing a record.
func TestMessageSaveWorkerRejectsUnknownActions(t *testing.T) {
	t.Parallel()

	worker := messageSaveWorker{}

	require.EqualError(t, worker.save(context.Background(), queue.Action{Name: "unknown"}), `unsupported action "unknown"`)
	require.EqualError(t, worker.save(context.Background(), queue.Action{Name: queue.ActionSaveMessage}), "missing chatId for saveMessage")
}

// saveRawMessage builds a queue record without including Telegram message text.
func saveRawMessage(t *testing.T, receiptHandle string, messageID string, date string) queue.RawMessage {
	t.Helper()
	return mustRawMessage(t, `{"receiptHandle":"`+receiptHandle+`","messageAttributes":{"action":{"StringValue":"saveMessage"},"chatId":{"StringValue":"42"},"chatTitle":{"StringValue":"Group"},"userId":{"StringValue":"7"},"messageId":{"StringValue":"`+messageID+`"},"date":{"StringValue":"`+date+`"}}}`)
}
