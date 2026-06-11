package integration

import (
	"context"
	"strconv"
	"testing"
	"time"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	appdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/service"
	"github.com/siutsin/telegram-jung2-bot/internal/statistics"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

const (
	workerChatID    int64 = 42003
	workerChatTitle       = "Worker Integration"
	workerUserID    int64 = 10003
)

func runWorkerRunIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	messenger := &recordingMessenger{}
	svc := newIntegrationService(dynamoClient, sqsClient, resources, messenger)
	queueClient := queue.NewClient(sqsClient)

	queueWorker, err := worker.NewPollingWorker(
		resources.queueURL,
		queueClient,
		queueClient,
		buildWorkerHandlers(svc),
	)
	require.NoError(t, err, "create polling worker")

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- queueWorker.Run(workerCtx)
	}()

	action := mustCommandAction(t, "/jungHelp", workerChatID, workerChatTitle, workerUserID)
	producer := queue.NewProducer(resources.queueURL, queueClient)
	err = producer.Enqueue(ctx, action)
	require.NoError(t, err, "enqueue worker run action")

	require.Eventually(t, func() bool {
		return len(messenger.recordedMessages()) > 0
	}, 15*time.Second, 100*time.Millisecond, "worker run should dispatch jungHelp")

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, workerChatID, messages[0].chatID)
	assert.Contains(t, messages[0].text, statistics.HelpMessage(workerChatTitle))

	cancel()

	select {
	case runErr := <-done:
		if workerCtx.Err() != nil {
			// Cancel during an in-flight SQS long poll returns context.Canceled;
			// app.Run normalises that through normaliseWorkerRunError.
			break
		}
		require.NoError(t, runErr, "worker run should stop after cancel")
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for worker run to stop")
	}

	assertQueueEmpty(t, ctx, queueClient, resources.queueURL)
}

func buildWorkerHandlers(svc service.Service) worker.Handlers {
	return worker.Handlers{
		JungHelp:       svc.JungHelp,
		TopTen:         svc.TopTen,
		TopDiver:       svc.TopDiver,
		AllJung:        svc.AllJung,
		OffFromWork:    svc.OffFromWork,
		EnableAllJung:  svc.EnableAllJung,
		DisableAllJung: svc.DisableAllJung,
		SetOffWorkTime: svc.SetOffWorkTime,
		OnOffFromWork:  svc.OnOffFromWork,
	}
}

func runWorkerServiceIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	drainQueue(t, ctx, queue.NewClient(sqsClient), resources.queueURL)

	t.Run("jungHelp dispatch", func(t *testing.T) {
		runWorkerJungHelpCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("topTen dispatch", func(t *testing.T) {
		runWorkerTopTenCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("offFromWork dispatch", func(t *testing.T) {
		runWorkerOffFromWorkCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("topDiver dispatch", func(t *testing.T) {
		runWorkerTopDiverCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("allJung dispatch", func(t *testing.T) {
		runWorkerAllJungCase(t, ctx, dynamoClient, sqsClient, resources)
	})
}

func runWorkerJungHelpCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	messenger := &recordingMessenger{}
	action := mustCommandAction(t, "/jungHelp", workerChatID, workerChatTitle, workerUserID)
	pollServiceAction(t, ctx, dynamoClient, sqsClient, resources, messenger, action)

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, workerChatID, messages[0].chatID)
	assert.Contains(t, messages[0].text, statistics.HelpMessage(workerChatTitle))
	assert.True(t, messages[0].options.DisableWebPagePreview)
	assert.Equal(t, "markdown", messages[0].options.ParseMode)
}

func runWorkerTopTenCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	seedWorkerTopTenData(t, ctx, dynamoClient, resources)

	messenger := &recordingMessenger{}
	action := mustCommandAction(t, "/topTen", workerChatID, workerChatTitle, workerUserID)
	pollServiceAction(t, ctx, dynamoClient, sqsClient, resources, messenger, action)

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, workerChatID, messages[0].chatID)
	assert.Contains(t, messages[0].text, workerChatTitle)
	assert.Contains(t, messages[0].text, "Top 10 冗員s")
}

func runWorkerOffFromWorkCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	seedWorkerTopTenData(t, ctx, dynamoClient, resources)

	messenger := &recordingMessenger{}
	action := queue.Action{
		Name: queue.ActionOffFromWork,
		Body: queue.BodyOffFromWork,
		Attributes: map[string]string{
			"chatId": formatInt(workerChatID),
			"action": queue.ActionOffFromWork,
		},
	}
	pollServiceAction(t, ctx, dynamoClient, sqsClient, resources, messenger, action)

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, workerChatID, messages[0].chatID)
	assert.Contains(t, messages[0].text, "夠鐘收工")
}

func runWorkerTopDiverCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	seedWorkerTopTenData(t, ctx, dynamoClient, resources)

	messenger := &recordingMessenger{}
	action := mustCommandAction(t, "/topDiver", workerChatID, workerChatTitle, workerUserID)
	pollServiceAction(t, ctx, dynamoClient, sqsClient, resources, messenger, action)

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, workerChatID, messages[0].chatID)
	assert.Contains(t, messages[0].text, "潛水員s")
}

func runWorkerAllJungCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	seedWorkerTopTenData(t, ctx, dynamoClient, resources)

	messenger := &recordingMessenger{}
	action := mustCommandAction(t, "/allJung", workerChatID, workerChatTitle, workerUserID)
	pollServiceAction(t, ctx, dynamoClient, sqsClient, resources, messenger, action)

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, workerChatID, messages[0].chatID)
	assert.Contains(t, messages[0].text, "All 冗員s")
}

func seedWorkerTopTenData(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	resources testResources,
) {
	t.Helper()

	chatRepo := appdynamodb.NewChatClient(dynamoClient)
	messageRepo := appdynamodb.NewMessageClient(dynamoClient)

	settings := chat.ChatSetting{
		ChatID:        workerChatID,
		ChatTitle:     workerChatTitle,
		DateCreated:   integrationNow,
		TTL:           message.TTL(integrationNow, message.DefaultTTL),
		EnableAllJung: true,
	}
	err := chatRepo.Save(ctx, resources.chatTable, settings)
	require.NoError(t, err, "seed worker chat row")

	users := []struct {
		firstName string
		offset    time.Duration
	}{
		{firstName: "Alice", offset: 3 * time.Minute},
		{firstName: "Bob", offset: 2 * time.Minute},
		{firstName: "Carol", offset: time.Minute},
	}
	for _, user := range users {
		row := message.Message{
			ChatID:      workerChatID,
			DateCreated: integrationNow.Add(user.offset),
			ChatTitle:   workerChatTitle,
			UserID:      workerUserID,
			FirstName:   user.firstName,
			TTL:         message.TTL(integrationNow, message.DefaultTTL),
		}
		err = messageRepo.Save(ctx, resources.messageTable, row)
		require.NoError(t, err, "seed worker message for %s", user.firstName)
	}
}

func pollServiceAction(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
	messenger *recordingMessenger,
	action queue.Action,
) {
	t.Helper()

	queueClient := queue.NewClient(sqsClient)
	producer := queue.NewProducer(resources.queueURL, queueClient)
	svc := newIntegrationService(dynamoClient, sqsClient, resources, messenger)

	err := producer.Enqueue(ctx, action)
	require.NoError(t, err, "enqueue worker action")

	err = queue.NewConsumer(resources.queueURL, queueClient).Poll(ctx, func(pollCtx context.Context, raw queue.RawMessage) error {
		decoded := queue.DecodeMessage(raw)
		handlerErr := dispatchAllServiceActions(pollCtx, svc, decoded)
		if handlerErr != nil {
			return handlerErr
		}

		return queueClient.Delete(pollCtx, queue.DeleteMessageRequest{
			QueueURL:      resources.queueURL,
			ReceiptHandle: raw.ReceiptHandle,
		})
	})
	require.NoError(t, err, "poll queue action")

	assertQueueEmpty(t, ctx, queueClient, resources.queueURL)
}

func newIntegrationService(
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
	messenger *recordingMessenger,
) service.Service {
	chatRepo := appdynamodb.NewChatClient(dynamoClient)
	messageRepo := appdynamodb.NewMessageClient(dynamoClient)
	sender := queue.NewClient(sqsClient)

	return service.New(
		chatRepo,
		resources.chatTable,
		messageRepo,
		resources.messageTable,
		messenger,
		func() time.Time { return integrationNow },
		resources.queueURL,
		sender,
	)
}

func actionChatID(action queue.Action) int64 {
	return actionChatIDFromAttribute(action.Attributes["chatId"])
}

func actionChatIDFromAttribute(raw string) int64 {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}

	return value
}
