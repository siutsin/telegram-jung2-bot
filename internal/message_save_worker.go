package bot

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=message_save_worker.go -destination=message_save_worker_mock_test.go -package=bot -mock_names storedMessageSaver=MockStoredMessageSaver,storedChatSaver=MockStoredChatSaver,queueBatchDeleter=MockQueueBatchDeleter"

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

type storedMessageSaver interface {
	Save(ctx context.Context, tableName string, row StoredMessage) error
}

type storedChatSaver interface {
	Save(ctx context.Context, tableName string, settings ChatSetting) error
}

type queueBatchDeleter interface {
	DeleteBatch(ctx context.Context, request queue.DeleteMessageBatchRequest) error
}

// messageSaveWorker writes FIFO message-save actions and deletes each received
// batch together.
type messageSaveWorker struct {
	queueURL     string
	receiver     queueReceiver
	deleter      queueBatchDeleter
	messages     storedMessageSaver
	messageTable string
	chats        storedChatSaver
	chatTable    string
	now          func() time.Time
	metrics      *Metrics
}

// NewMessageSaveWorker builds the dedicated FIFO message-save worker.
func NewMessageSaveWorker(
	queueURL string,
	receiver queueReceiver,
	deleter queueBatchDeleter,
	messages storedMessageSaver,
	messageTable string,
	chats storedChatSaver,
	chatTable string,
	now func() time.Time,
	metrics *Metrics,
) (messageSaveWorker, error) {
	if receiver == nil {
		return messageSaveWorker{}, fmt.Errorf("queue receiver is required")
	}
	if deleter == nil {
		return messageSaveWorker{}, fmt.Errorf("queue batch deleter is required")
	}
	if messages == nil {
		return messageSaveWorker{}, fmt.Errorf("message store is required")
	}
	if chats == nil {
		return messageSaveWorker{}, fmt.Errorf("chat store is required")
	}

	return messageSaveWorker{
		queueURL:     queueURL,
		receiver:     receiver,
		deleter:      deleter,
		messages:     messages,
		messageTable: messageTable,
		chats:        chats,
		chatTable:    chatTable,
		now:          now,
		metrics:      metrics,
	}, nil
}

// Run polls the FIFO queue until the context is cancelled.
func (worker messageSaveWorker) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		response, err := worker.receiver.ReceiveMessage(ctx, queue.ReceiveMessageRequest{
			QueueURL:            worker.queueURL,
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			return err
		}

		receipts := make([]string, 0, len(response.Messages))
		for _, raw := range response.Messages {
			action := queue.DecodeMessage(raw)
			(pollingWorker{metrics: worker.metrics}).recordQueueWait(action, time.Now())
			err = worker.save(ctx, action)
			if err != nil {
				slog.Error("save queued message", "err", err)
				continue
			}
			receipts = append(receipts, raw.ReceiptHandle)
		}
		if len(receipts) == 0 {
			continue
		}
		err = worker.deleter.DeleteBatch(ctx, queue.DeleteMessageBatchRequest{
			QueueURL:       worker.queueURL,
			ReceiptHandles: receipts,
		})
		if err != nil {
			return err
		}
	}
}

// save keeps a failed record visible in SQS so DynamoDB can retry it.
func (worker messageSaveWorker) save(ctx context.Context, action queue.Action) error {
	if action.Name != queue.ActionSaveMessage {
		return fmt.Errorf("unsupported action %q", action.Name)
	}

	message, err := telegramMessageFromAction(action)
	if err != nil {
		return err
	}
	now := time.Now()
	if worker.now != nil {
		now = worker.now()
	}
	err = worker.messages.Save(ctx, worker.messageTable, StoredMessageFromTelegram(message, now))
	if err != nil {
		return fmt.Errorf("save message: %w", err)
	}
	err = worker.chats.Save(ctx, worker.chatTable, ChatFromTelegram(message, now))
	if err != nil {
		return fmt.Errorf("save chat: %w", err)
	}

	return nil
}

// telegramMessageFromAction rebuilds the message fields that the webhook puts
// in the queue. For example, date "1760000000" becomes a Telegram Unix time.
func telegramMessageFromAction(action queue.Action) (TelegramMessage, error) {
	chatID, err := queueActionInt64(action, "chatId")
	if err != nil {
		return TelegramMessage{}, err
	}
	messageID, err := queueActionInt64(action, "messageId")
	if err != nil {
		return TelegramMessage{}, err
	}
	date, err := queueActionInt64(action, "date")
	if err != nil || date <= 0 {
		return TelegramMessage{}, fmt.Errorf("invalid date for %s", action.Name)
	}

	message := TelegramMessage{
		MessageID: messageID,
		Date:      date,
		Chat: Chat{
			ID:    chatID,
			Title: action.Attributes["chatTitle"],
		},
	}
	if rawUserID := action.Attributes["userId"]; rawUserID != "" {
		userID, parseErr := strconv.ParseInt(rawUserID, 10, 64)
		if parseErr != nil {
			return TelegramMessage{}, fmt.Errorf("invalid userId for %s: %w", action.Name, parseErr)
		}
		message.From = &User{
			ID:        userID,
			UserName:  action.Attributes["username"],
			FirstName: action.Attributes["firstName"],
			LastName:  action.Attributes["lastName"],
		}
	}

	return message, nil
}

// queueActionInt64 rejects malformed queue data before it reaches storage.
func queueActionInt64(action queue.Action, key string) (int64, error) {
	raw := action.Attributes[key]
	if raw == "" {
		return 0, fmt.Errorf("missing %s for %s", key, action.Name)
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s for %s: %w", key, action.Name, err)
	}

	return value, nil
}
