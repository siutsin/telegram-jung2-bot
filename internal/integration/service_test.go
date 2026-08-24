package integration

import (
	"context"
	"testing"

	bot "github.com/siutsin/telegram-jung2-bot/internal"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/require"

	appdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

func runServiceOnOffFromWorkIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	seedOnOffFromWorkChat(t, ctx, dynamoClient, resources)

	queueClient := queue.NewClient(sqsClient)
	svc := newIntegrationService(dynamoClient, sqsClient, resources, &recordingMessenger{})

	err := svc.OnOffFromWork(ctx, "2026-06-11T18:30:00Z")
	require.NoError(t, err, "fan out due off-work actions")

	queueResponse, err := receiveOne(ctx, queueClient, resources.queueURL)
	require.NoError(t, err, "receive offFromWork queue message")

	gotAction := queue.DecodeMessage(queueResponse.Messages[0])
	wantAction := bot.BuildOffFromWorkAction(integrationChatID)
	assertAction(t, wantAction, gotAction)

	err = queueClient.Delete(ctx, queue.DeleteMessageRequest{
		QueueURL:      resources.queueURL,
		ReceiptHandle: queueResponse.Messages[0].ReceiptHandle,
	})
	require.NoError(t, err, "delete offFromWork queue message")
}

func seedOnOffFromWorkChat(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	resources testResources,
) {
	t.Helper()

	chatRepo := appdynamodb.NewChatClient(dynamoClient)
	settings := bot.ChatSetting{
		ChatID:      integrationChatID,
		ChatTitle:   integrationChatTitle,
		DateCreated: integrationNow,
		TTL:         bot.TTL(integrationNow, bot.DefaultTTL),
	}
	err := chatRepo.Save(ctx, resources.chatTable, settings)
	require.NoError(t, err, "seed onOffFromWork chat")
	err = chatRepo.Update(ctx, bot.BuildOffWorkUpdate(resources.chatTable, settings.ChatID, "1830", bot.Thu))
	require.NoError(t, err, "seed onOffFromWork off-work settings")
}
