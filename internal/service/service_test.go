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
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/statistics"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

func TestJungHelpSendsMarkdownHelp(t *testing.T) {
	t.Parallel()

	messenger := &fakeMessenger{}
	service := testService()
	service.messenger = messenger

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

	assert.Equal(t, chatStore, service.chatMaintainer)
	assert.Equal(t, "chats", service.chatTable)
	assert.Equal(t, messageClient, service.messageQuerier)
	assert.Equal(t, "messages", service.messageTable)
	assert.Equal(t, messenger, service.messenger)
	assert.Equal(t, now, service.nowFunc())
	assert.Equal(t, "queue-url", service.queueURL)
	assert.Equal(t, sender, service.sender)
}

func TestOnOffFromWorkEnqueuesDueChats(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	service := testService()
	service.chatMaintainer = &fakeChatStore{dueChatIDs: []int64{123}}
	service.sender = sender

	err := service.OnOffFromWork(context.Background(), "2026-05-01T18:00:00+01:00")

	require.NoError(t, err)
	require.Len(t, sender.requests, 1)
	assert.Equal(t, queue.BodyOffFromWork, sender.requests[0].MessageBody)
	assert.Equal(t, "123", sender.requests[0].MessageAttributes["chatId"].StringValue)
}

func TestAllJungHonoursEnableSetting(t *testing.T) {
	t.Parallel()

	disabled := false
	service := testService()
	service.chatMaintainer = &fakeChatStore{enabled: &disabled}
	service.messageQuerier = &fakeMessageClient{err: errors.New("should not query")}

	err := service.AllJung(context.Background(), 123)

	require.NoError(t, err)
}

func TestAllJungSendsReportWhenChatHasNoStoredSetting(t *testing.T) {
	t.Parallel()

	messenger := &fakeMessenger{}
	service := testService()
	service.messageQuerier = &fakeMessageClient{rows: serviceRows()}
	service.messenger = messenger

	err := service.AllJung(context.Background(), 123)

	require.NoError(t, err)
	assert.Contains(t, messenger.text, "All 冗員s")
}

func TestAllJungReturnsChatLookupError(t *testing.T) {
	t.Parallel()

	service := testService()
	service.chatMaintainer = &fakeChatStore{getErr: errors.New("boom")}

	err := service.AllJung(context.Background(), 123)

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestEnableAndDisableAllJungApplyAdminSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		run       func(Service) error
		wantReply string
	}{
		{
			name: "enable",
			run: func(service Service) error {
				return service.EnableAllJung(context.Background(), 123, "Group", 456)
			},
			wantReply: "Enabled AllJung command",
		},
		{
			name: "disable",
			run: func(service Service) error {
				return service.DisableAllJung(context.Background(), 123, "Group", 456)
			},
			wantReply: "Disabled AllJung command",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			messenger := &fakeMessenger{}
			chatStore := &fakeChatStore{}
			service := testService()
			service.chatMaintainer = chatStore
			service.messenger = messenger

			err := test.run(service)

			require.NoError(t, err)
			assert.Equal(t, int64(123), chatStore.updatedChatID)
			assert.Contains(t, messenger.text, test.wantReply)
		})
	}
}

func TestEnableAndDisableAllJungReturnAdminLookupErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(Service) error
	}{
		{
			name: "enable",
			run: func(service Service) error {
				return service.EnableAllJung(context.Background(), 123, "Group", 456)
			},
		},
		{
			name: "disable",
			run: func(service Service) error {
				return service.DisableAllJung(context.Background(), 123, "Group", 456)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := testService()
			service.messenger = &fakeMessenger{adminErr: errors.New("boom")}

			err := test.run(service)

			require.Error(t, err)
			assert.EqualError(t, err, "boom")
		})
	}
}

func TestSettingChangeSkipsNonAdminAndReturnsUpdateError(t *testing.T) {
	t.Parallel()

	admin := false
	messenger := &fakeMessenger{admin: &admin}
	chatStore := &fakeChatStore{}
	service := testService()
	service.chatMaintainer = chatStore
	service.messenger = messenger

	err := service.EnableAllJung(context.Background(), 123, "Group", 456)

	require.NoError(t, err)
	assert.Zero(t, chatStore.updatedChatID)
	assert.Empty(t, messenger.text)

	service = testService()
	service.chatMaintainer = &fakeChatStore{updateErr: errors.New("boom")}

	err = service.EnableAllJung(context.Background(), 123, "Group", 456)

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestOffFromWorkAndTopDiverSendReports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		run      func(Service) error
		wantText string
	}{
		{
			name: "off from work",
			run: func(service Service) error {
				return service.OffFromWork(context.Background(), 123)
			},
			wantText: "夠鐘收工~~",
		},
		{
			name: "top diver",
			run: func(service Service) error {
				return service.TopDiver(context.Background(), 123)
			},
			wantText: "Top 10 潛水員s",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			messenger := &fakeMessenger{}
			service := testService()
			service.messageQuerier = &fakeMessageClient{rows: serviceRows()}
			service.messenger = messenger

			err := test.run(service)

			require.NoError(t, err)
			assert.Contains(t, messenger.text, test.wantText)
		})
	}
}

func TestParseScheduledTimeRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := schedule.ParseScheduledTime("bad")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse scheduled time")
}

func TestOnOffFromWorkSkipsInvalidScheduledTime(t *testing.T) {
	t.Parallel()

	err := testService().OnOffFromWork(context.Background(), "bad")

	require.NoError(t, err)
}

func TestOnOffFromWorkReturnsFanOutErrors(t *testing.T) {
	t.Parallel()

	service := testService()
	service.chatMaintainer = &fakeChatStore{dueErr: errors.New("boom")}

	err := service.OnOffFromWork(context.Background(), "2026-05-01T18:00:00Z")

	require.Error(t, err)
	require.EqualError(t, err, "boom")

	service = testService()
	service.chatMaintainer = &fakeChatStore{dueChatIDs: []int64{123}}
	service.sender = &fakeSender{err: errors.New("boom")}

	err = service.OnOffFromWork(context.Background(), "2026-05-01T18:00:00Z")

	require.Error(t, err)
	require.EqualError(t, err, "enqueue due off-work report: boom")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service = testService()
	service.chatMaintainer = &fakeChatStore{dueChatIDs: []int64{123}}

	err = service.OnOffFromWork(ctx, "2026-05-01T18:00:00Z")

	require.ErrorIs(t, err, context.Canceled)
}

func TestPauseFanOutReturnsContextError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pauseFanOut(ctx, time.Hour)

	require.ErrorIs(t, err, context.Canceled)
}

func TestTopTenIgnoresTelegramStatusErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	messenger := &fakeMessenger{err: errors.New("telegram API returned HTTP 403")}
	chatStore := &fakeChatStore{}
	service := testService()
	service.chatMaintainer = chatStore
	service.messageQuerier = &fakeMessageClient{
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
	service.messenger = messenger
	service.nowFunc = func() time.Time { return now }

	err := service.TopTen(context.Background(), 123)

	require.NoError(t, err)
	assert.Equal(t, statisticsChatCountUpdate(now), chatStore.statisticsUpdate)
}

func TestTopTenReturnsStatisticsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		service Service
		wantErr string
	}{
		{
			name: "query",
			service: func() Service {
				service := testService()
				service.messageQuerier = &fakeMessageClient{err: errors.New("boom")}
				return service
			}(),
			wantErr: "boom",
		},
		{
			name:    "empty rows",
			service: testService(),
		},
		{
			name: "save statistics",
			service: func() Service {
				service := testService()
				service.chatMaintainer = &fakeChatStore{saveErr: errors.New("boom")}
				service.messageQuerier = &fakeMessageClient{rows: serviceRows()}
				return service
			}(),
			wantErr: "boom",
		},
		{
			name: "send",
			service: func() Service {
				service := testService()
				service.messageQuerier = &fakeMessageClient{rows: serviceRows()}
				service.messenger = &fakeMessenger{err: errors.New("boom")}
				return service
			}(),
			wantErr: "boom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.service.TopTen(context.Background(), 123)

			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
}

func TestOffFromWorkSkipsEmptyChatWindow(t *testing.T) {
	t.Parallel()

	messenger := &fakeMessenger{}
	service := testService()
	service.messenger = messenger

	err := service.OffFromWork(context.Background(), 123)

	require.NoError(t, err)
	assert.Empty(t, messenger.text)
}

func TestSetOffWorkTimeUsesWorkerInput(t *testing.T) {
	t.Parallel()

	messenger := &fakeMessenger{}
	chatStore := &fakeChatStore{}
	service := testService()
	service.chatMaintainer = chatStore
	service.messenger = messenger

	err := service.SetOffWorkTime(context.Background(), schedule.SetOffInput{
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

func TestSetOffWorkTimeSkipsInvalidInputForNonAdmin(t *testing.T) {
	t.Parallel()

	admin := false
	messenger := &fakeMessenger{admin: &admin}
	service := testService()
	service.messenger = messenger

	err := service.SetOffWorkTime(context.Background(), schedule.SetOffInput{
		ChatID:    123,
		ChatTitle: "Group",
		UserID:    456,
		OffTime:   "1800",
		Workday:   "BAD",
	})

	require.NoError(t, err)
	assert.Empty(t, messenger.text)
}

func TestSetOffWorkTimeReturnsInvalidReplySendError(t *testing.T) {
	t.Parallel()

	service := testService()
	service.messenger = &fakeMessenger{err: errors.New("boom")}

	err := service.SetOffWorkTime(context.Background(), schedule.SetOffInput{
		ChatID:    123,
		ChatTitle: "Group",
		UserID:    456,
		OffTime:   "1800",
		Workday:   "BAD",
	})

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestSetOffWorkTimeIgnoresTelegramStatusErrorOnInvalidReply(t *testing.T) {
	t.Parallel()

	service := testService()
	service.messenger = &fakeMessenger{err: errors.New("telegram API returned HTTP 403")}

	err := service.SetOffWorkTime(context.Background(), schedule.SetOffInput{
		ChatID:    123,
		ChatTitle: "Group",
		UserID:    456,
		OffTime:   "1800",
		Workday:   "BAD",
	})

	require.NoError(t, err)
}

func TestApplySettingChangeIgnoresTelegramStatusError(t *testing.T) {
	t.Parallel()

	messenger := &fakeMessenger{err: errors.New("telegram API returned HTTP 500")}
	chatStore := &fakeChatStore{}
	service := testService()
	service.chatMaintainer = chatStore
	service.messenger = messenger

	err := service.EnableAllJung(context.Background(), 123, "Group", 456)

	require.NoError(t, err)
	assert.Equal(t, int64(123), chatStore.updatedChatID)
}

func TestSetOffWorkTimeReturnsValidationAndAdminErrors(t *testing.T) {
	t.Parallel()

	service := testService()
	service.messenger = &fakeMessenger{adminErr: errors.New("boom")}

	err := service.SetOffWorkTime(context.Background(), schedule.SetOffInput{ChatID: 123, UserID: 456})

	require.Error(t, err)
	require.EqualError(t, err, "boom")

	messenger := &fakeMessenger{}
	service = testService()
	service.messenger = messenger
	err = service.SetOffWorkTime(context.Background(), schedule.SetOffInput{
		ChatID:    123,
		ChatTitle: "Group",
		UserID:    456,
		OffTime:   "1800",
		Workday:   "BAD",
	})

	require.NoError(t, err)
	assert.Contains(t, messenger.text, "Invalid format for setOffFromWorkTimeUTC")
}

func TestServiceNowDefaultsToCurrentTime(t *testing.T) {
	t.Parallel()

	service := testService()
	service.nowFunc = nil
	before := time.Now()

	got := service.now()

	assert.WithinDuration(t, before, got, time.Second)
}

func TestIsTelegramStatusErrorHandlesNilAndServerErrors(t *testing.T) {
	t.Parallel()

	assert.False(t, isTelegramStatusError(nil))
	assert.True(t, isTelegramStatusError(errors.New("telegram API returned HTTP 500")))
	assert.True(t, isTelegramStatusError(errors.New("telegram API returned HTTP 429")))
	assert.True(t, isTelegramStatusError(errors.New("telegram API returned HTTP 403")))
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
		chatMaintainer: &fakeChatStore{},
		chatTable:      "chats",
		messageQuerier: &fakeMessageClient{},
		messageTable:   "messages",
		messenger:      &fakeMessenger{},
		nowFunc: func() time.Time {
			return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		},
		queueURL: "https://example.com/queue",
		sender:   &fakeSender{},
	}
}

func serviceRows() []message.Message {
	return []message.Message{
		{
			ChatID:      123,
			ChatTitle:   "Group",
			DateCreated: time.Date(2026, 5, 2, 20, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)),
			FirstName:   "Ada",
			TTL:         1,
			UserID:      1,
		},
		{
			ChatID:      123,
			ChatTitle:   "Group",
			DateCreated: time.Date(2026, 5, 1, 20, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)),
			FirstName:   "Grace",
			TTL:         1,
			UserID:      2,
		},
	}
}

type fakeChatStore struct {
	dueChatIDs       []int64
	dueErr           error
	enabled          *bool
	getErr           error
	saveErr          error
	statisticsUpdate chat.UpdateExpression
	updateErr        error
	updatedChatID    int64
}

func (store *fakeChatStore) DueChatIDs(ctx context.Context, tableName string, timestamp time.Time) ([]int64, error) {
	if store.dueErr != nil {
		return nil, store.dueErr
	}

	return append([]int64(nil), store.dueChatIDs...), nil
}

func (store *fakeChatStore) Get(ctx context.Context, tableName string, chatID int64) (chat.ChatSetting, bool, error) {
	if store.getErr != nil {
		return chat.ChatSetting{}, false, store.getErr
	}
	if store.enabled == nil {
		return chat.ChatSetting{}, false, nil
	}

	return chat.ChatSetting{EnableAllJung: *store.enabled}, true, nil
}

func (store *fakeChatStore) SaveStatistics(ctx context.Context, tableName string, chatID int64, userCount int, messageCount int, now time.Time) error {
	if store.saveErr != nil {
		return store.saveErr
	}

	store.statisticsUpdate = statisticsChatCountUpdate(now)
	return nil
}

func (store *fakeChatStore) Update(ctx context.Context, request chat.UpdateExpression) error {
	if store.updateErr != nil {
		return store.updateErr
	}

	chatID, ok := request.Key["chatId"].(int64)
	if !ok {
		return errors.New("chatId type mismatch")
	}
	store.updatedChatID = chatID
	return nil
}

type fakeMessageClient struct {
	err  error
	rows []message.Message
}

func (client *fakeMessageClient) QueryByChat(ctx context.Context, tableName string, chatID int64, since time.Time) ([]message.Message, error) {
	if client.err != nil {
		return nil, client.err
	}

	return append([]message.Message(nil), client.rows...), nil
}

type fakeMessenger struct {
	admin    *bool
	adminErr error
	chatID   int64
	err      error
	options  telegram.SendMessageOptions
	text     string
}

func (messenger *fakeMessenger) IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error) {
	if messenger.adminErr != nil {
		return false, messenger.adminErr
	}
	if messenger.admin == nil {
		return true, nil
	}

	return *messenger.admin, nil
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
	err      error
	requests []queue.SendMessageRequest
}

func (sender *fakeSender) SendMessage(ctx context.Context, request queue.SendMessageRequest) error {
	sender.requests = append(sender.requests, request)
	return sender.err
}
