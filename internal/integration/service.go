package integration

import (
	"context"
	"testing"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
)

func runServiceOnOffFromWorkIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	queueClient := queue.NewClient(sqsClient)
	drainQueue(t, ctx, queueClient, resources.queueURL)
	svc := newIntegrationService(dynamoClient, sqsClient, resources, &recordingMessenger{})

	err := svc.OnOffFromWork(ctx, "2026-06-11T18:30:00Z")
	require.NoError(t, err, "fan out due off-work actions")

	queueResponse, err := receiveOne(ctx, queueClient, resources.queueURL)
	require.NoError(t, err, "receive offFromWork queue message")

	gotAction := queue.DecodeMessage(queueResponse.Messages[0])
	wantAction := schedule.BuildOffFromWorkAction(integrationChatID)
	assertAction(t, wantAction, gotAction)

	err = queueClient.Delete(ctx, queue.DeleteMessageRequest{
		QueueURL:      resources.queueURL,
		ReceiptHandle: queueResponse.Messages[0].ReceiptHandle,
	})
	require.NoError(t, err, "delete offFromWork queue message")
}
