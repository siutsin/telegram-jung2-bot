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
