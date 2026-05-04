package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/statistics"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

func TestJungHelpSendsMarkdownHelp(t *testing.T) {
	t.Parallel()

	messenger := &fakeMessenger{}
	service := testService()
	service.Messenger = messenger

	err := service.JungHelp(context.Background(), 123, "Group")

	require.NoError(t, err)
	assert.Equal(t, int64(123), messenger.chatID)
	assert.Equal(t, statistics.HelpMessage("Group"), messenger.text)
	assert.Equal(t, telegram.SendMessageOptions{
		DisableWebPagePreview: true,
		ParseMode:             "markdown",
	}, messenger.options)
}

func TestNewBuildsService(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	chatStore := &fakeChatStore{}
	messageClient := &fakeMessageClient{}
	messenger := &fakeMessenger{}
	sender := &fakeSender{}

	service := New(
		chatStore,
		"chats",
		messageClient,
		"messages",
		messenger,
		func() time.Time { return now },
		"queue-url",
		sender,
	)

	assert.Equal(t, chatStore, service.ChatMaintainer)
	assert.Equal(t, "chats", service.ChatTable)
	assert.Equal(t, messageClient, service.MessageQuerier)
	assert.Equal(t, "messages", service.MessageTable)
	assert.Equal(t, messenger, service.Messenger)
	assert.Equal(t, now, service.Now())
	assert.Equal(t, "queue-url", service.QueueURL)
	assert.Equal(t, sender, service.Sender)
}

func TestOnOffFromWorkEnqueuesDueChats(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	service := testService()
	service.ChatMaintainer = &fakeChatStore{dueChatIDs: []int64{123}}
	service.Sender = sender

	err := service.OnOffFromWork(context.Background(), "2026-05-01T18:00:00+01:00")

	require.NoError(t, err)
	require.Len(t, sender.requests, 1)
	assert.Equal(t, queue.BodyOffFromWork, sender.requests[0].MessageBody)
	assert.Equal(t, "123", sender.requests[0].MessageAttributes["chatId"].StringValue)
}

func TestParseScheduledTimeRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := parseScheduledTime("bad")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse scheduled time")
}

func TestTopTenIgnoresTelegramStatusErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	messenger := &fakeMessenger{err: errors.New("telegram API returned HTTP 403")}
	chatStore := &fakeChatStore{}
	service := testService()
	service.ChatMaintainer = chatStore
	service.MessageQuerier = &fakeMessageClient{
		rows: []message.Message{
			{
				ChatID:      123,
				ChatTitle:   "Group",
				DateCreated: time.Date(2026, 5, 2, 20, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)),
				FirstName:   "Ada",
				TTL:         1,
				UserID:      1,
			},
		},
	}
	service.Messenger = messenger
	service.Now = func() time.Time { return now }

	err := service.TopTen(context.Background(), 123)

	require.NoError(t, err)
	assert.Equal(t, statisticsChatCountUpdate(now), chatStore.statisticsUpdate)
}

func TestSetOffWorkTimeUsesWorkerInput(t *testing.T) {
	t.Parallel()

	messenger := &fakeMessenger{}
	chatStore := &fakeChatStore{}
	service := testService()
	service.ChatMaintainer = chatStore
	service.Messenger = messenger

	err := service.SetOffWorkTime(context.Background(), worker.SetOffInput{
		ChatID:    123,
		ChatTitle: "Group",
		UserID:    456,
		OffTime:   "1800",
		Workday:   "MON,WED",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(123), chatStore.updatedChatID)
	assert.Contains(t, messenger.text, "Updated setOffFromWorkTime in UTC: 1800 MON,WED")
}

func statisticsChatCountUpdate(now time.Time) chat.UpdateExpression {
	return chat.UpdateExpression{
		TableName:        "chats",
		Key:              map[string]any{"chatId": int64(123)},
		UpdateExpression: "SET #uc = :uc, #mc = :mc, #mpu = :mpu, #ct = :ct",
		ExpressionAttributeNames: map[string]string{
			"#uc":  "userCount",
			"#mc":  "messageCount",
			"#mpu": "messagePerUser",
			"#ct":  "countTimestamp",
		},
		ExpressionAttributeValues: map[string]any{
			":uc":  1,
			":mc":  1,
			":mpu": 1.0,
			":ct":  message.FormatDateCreated(now),
		},
	}
}

func testService() Service {
	return Service{
		ChatMaintainer: &fakeChatStore{},
		ChatTable:      "chats",
		MessageQuerier: &fakeMessageClient{},
		MessageTable:   "messages",
		Messenger:      &fakeMessenger{},
		Now: func() time.Time {
			return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		},
		QueueURL: "https://example.com/queue",
		Sender:   &fakeSender{},
	}
}

type fakeChatStore struct {
	dueChatIDs       []int64
	enabled          *bool
	statisticsUpdate chat.UpdateExpression
	updatedChatID    int64
}

func (store *fakeChatStore) DueChatIDs(ctx context.Context, tableName string, timestamp time.Time) ([]int64, error) {
	return append([]int64(nil), store.dueChatIDs...), nil
}

func (store *fakeChatStore) Get(ctx context.Context, tableName string, chatID int64) (chat.ChatSetting, bool, error) {
	if store.enabled == nil {
		return chat.ChatSetting{}, false, nil
	}

	return chat.ChatSetting{EnableAllJung: *store.enabled}, true, nil
}

func (store *fakeChatStore) SaveStatistics(ctx context.Context, tableName string, chatID int64, userCount int, messageCount int, now time.Time) error {
	store.statisticsUpdate = statisticsChatCountUpdate(now)
	return nil
}

func (store *fakeChatStore) Update(ctx context.Context, request chat.UpdateExpression) error {
	chatID, ok := request.Key["chatId"].(int64)
	if !ok {
		return errors.New("chatId type mismatch")
	}
	store.updatedChatID = chatID
	return nil
}

type fakeMessageClient struct {
	rows []message.Message
}

func (client *fakeMessageClient) QueryByChat(ctx context.Context, tableName string, chatID int64, since time.Time) ([]message.Message, error) {
	return append([]message.Message(nil), client.rows...), nil
}

type fakeMessenger struct {
	chatID  int64
	err     error
	options telegram.SendMessageOptions
	text    string
}

func (messenger *fakeMessenger) IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error) {
	return true, nil
}

func (messenger *fakeMessenger) SendMessage(ctx context.Context, chatID int64, text string) error {
	messenger.chatID = chatID
	messenger.text = text
	return messenger.err
}

func (messenger *fakeMessenger) SendMessageWithOptions(ctx context.Context, chatID int64, text string, options telegram.SendMessageOptions) error {
	messenger.chatID = chatID
	messenger.text = text
	messenger.options = options
	return messenger.err
}

type fakeSender struct {
	requests []queue.SendMessageRequest
}

func (sender *fakeSender) SendMessage(ctx context.Context, request queue.SendMessageRequest) error {
	sender.requests = append(sender.requests, request)
	return nil
}
