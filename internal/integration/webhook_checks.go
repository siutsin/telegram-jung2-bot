package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/command"
	appdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

const (
	webhookChatID    int64 = 42002
	webhookChatTitle       = "Webhook Integration"
	webhookUserID    int64 = 10002
)

type noopMessenger struct{}

func (noopMessenger) SendMessage(context.Context, int64, string) error {
	return nil
}

func runWebhookIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	now := time.Date(2026, 6, 11, 18, 30, 0, 0, time.UTC)
	queueClient := queue.NewClient(sqsClient)
	producer := queue.NewProducer(resources.queueURL, queueClient)

	server, err := httpserver.NewServer(
		":0",
		5*time.Second,
		"",
		httpserver.Dependencies{
			ChatTable:    resources.chatTable,
			MessageTable: resources.messageTable,
			Messages:     appdynamodb.NewMessageClient(dynamoClient),
			Chats:        appdynamodb.NewChatClient(dynamoClient),
			Enqueuer:     producer,
			Messenger:    noopMessenger{},
			Now: func() time.Time {
				return now
			},
		},
	)
	require.NoError(t, err, "create HTTP server")

	testServer := httptest.NewServer(server.Handler)
	t.Cleanup(testServer.Close)

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		testServer.URL+"/webhook",
		strings.NewReader(webhookTopTenPayload()),
	)
	require.NoError(t, err, "build webhook request")
	request.Header.Set("Content-Type", "application/json")

	httpResponse, err := http.DefaultClient.Do(request)
	require.NoError(t, err, "post webhook")
	defer func() {
		closeErr := httpResponse.Body.Close()
		if closeErr != nil {
			t.Errorf("close webhook response body: %v", closeErr)
		}
	}()
	assert.Equal(t, http.StatusOK, httpResponse.StatusCode)

	chatRepo := appdynamodb.NewChatClient(dynamoClient)
	messageRepo := appdynamodb.NewMessageClient(dynamoClient)

	gotChat, ok, err := chatRepo.Get(ctx, resources.chatTable, webhookChatID)
	require.NoError(t, err, "get webhook chat row")
	require.True(t, ok, "expected webhook chat row")
	assert.Equal(t, webhookChatTitle, gotChat.ChatTitle)
	assert.True(t, gotChat.EnableAllJung)

	messages, err := messageRepo.QueryByChat(ctx, resources.messageTable, webhookChatID, now.Add(-time.Minute))
	require.NoError(t, err, "query webhook message rows")
	require.Len(t, messages, 1)
	assert.Equal(t, webhookUserID, messages[0].UserID)
	assert.Equal(t, "floci-user", messages[0].Username)
	assert.Equal(t, "Floci", messages[0].FirstName)
	assert.Equal(t, "Tester", messages[0].LastName)
	assert.Equal(t, message.FormatDateCreated(now), message.FormatDateCreated(messages[0].DateCreated))

	wantAction, err := command.ActionFor(
		command.Command{Name: "topTen"},
		command.ChatContext{
			ChatID:    webhookChatID,
			ChatTitle: webhookChatTitle,
			UserID:    webhookUserID,
		},
	)
	require.NoError(t, err, "build expected topTen action")

	queueResponse, err := receiveOne(ctx, queueClient, resources.queueURL)
	require.NoError(t, err, "receive webhook queue message")

	gotAction := queue.DecodeMessage(queueResponse.Messages[0])
	assertAction(t, wantAction, gotAction)

	err = queueClient.Delete(ctx, queue.DeleteMessageRequest{
		QueueURL:      resources.queueURL,
		ReceiptHandle: queueResponse.Messages[0].ReceiptHandle,
	})
	require.NoError(t, err, "delete webhook queue message")
}

func webhookTopTenPayload() string {
	return `{"message":{"chat":{"id":42002,"title":"Webhook Integration","type":"supergroup"},"from":{"id":10002,"username":"floci-user","first_name":"Floci","last_name":"Tester"},"text":"/topTen","entities":[{"type":"bot_command"}]}}`
}
