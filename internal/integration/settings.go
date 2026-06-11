package integration

import (
	"context"
	"testing"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	appdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

const (
	settingsChatID    int64 = 42004
	settingsChatTitle       = "Settings Integration"
	settingsUserID    int64 = 10004
)

func runServiceAdminSettingsIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	drainQueue(t, ctx, queue.NewClient(sqsClient), resources.queueURL)
	seedSettingsChat(t, ctx, dynamoClient, resources)

	t.Run("disableAllJung", func(t *testing.T) {
		runDisableAllJungCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("enableAllJung", func(t *testing.T) {
		runEnableAllJungCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("setOffFromWorkTimeUTC", func(t *testing.T) {
		runSetOffWorkTimeCase(t, ctx, dynamoClient, sqsClient, resources)
	})
}

func runDisableAllJungCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	messenger := &recordingMessenger{admin: true}
	svc := newIntegrationService(dynamoClient, sqsClient, resources, messenger)

	err := svc.DisableAllJung(ctx, settingsChatID, settingsChatTitle, settingsUserID)
	require.NoError(t, err, "disable all jung")

	gotChat, ok, err := appdynamodb.NewChatClient(dynamoClient).Get(ctx, resources.chatTable, settingsChatID)
	require.NoError(t, err, "get chat after disable")
	require.True(t, ok)
	assert.False(t, gotChat.EnableAllJung)

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].text, "Disabled AllJung command")
}

func runEnableAllJungCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	messenger := &recordingMessenger{admin: true}
	svc := newIntegrationService(dynamoClient, sqsClient, resources, messenger)

	err := svc.EnableAllJung(ctx, settingsChatID, settingsChatTitle, settingsUserID)
	require.NoError(t, err, "enable all jung")

	gotChat, ok, err := appdynamodb.NewChatClient(dynamoClient).Get(ctx, resources.chatTable, settingsChatID)
	require.NoError(t, err, "get chat after enable")
	require.True(t, ok)
	assert.True(t, gotChat.EnableAllJung)

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].text, "Enabled AllJung command")
}

func runSetOffWorkTimeCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	messenger := &recordingMessenger{admin: true}
	svc := newIntegrationService(dynamoClient, sqsClient, resources, messenger)

	err := svc.SetOffWorkTime(ctx, worker.SetOffInput{
		ChatID:    settingsChatID,
		ChatTitle: settingsChatTitle,
		UserID:    settingsUserID,
		OffTime:   "1830",
		Workday:   "MON,TUE",
	})
	require.NoError(t, err, "set off-work time")

	gotChat, ok, err := appdynamodb.NewChatClient(dynamoClient).Get(ctx, resources.chatTable, settingsChatID)
	require.NoError(t, err, "get chat after setOff")
	require.True(t, ok)
	assert.Equal(t, "1830", gotChat.OffTime)
	assert.True(t, gotChat.HasOffTime)
	assert.True(t, workday.MatchesDay("MON", gotChat.Workday))
	assert.True(t, workday.MatchesDay("TUE", gotChat.Workday))

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].text, "1830")
	assert.Contains(t, messages[0].text, "MON")
}

func seedSettingsChat(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	resources testResources,
) {
	t.Helper()

	settings := chat.ChatSetting{
		ChatID:        settingsChatID,
		ChatTitle:     settingsChatTitle,
		DateCreated:   integrationNow,
		TTL:           message.TTL(integrationNow, message.DefaultTTL),
		EnableAllJung: true,
	}
	err := appdynamodb.NewChatClient(dynamoClient).Save(ctx, resources.chatTable, settings)
	require.NoError(t, err, "seed settings chat")
}
