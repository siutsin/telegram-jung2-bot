package integration

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/command"
	appdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
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

	t.Run("topTen command", func(t *testing.T) {
		runWebhookTopTenCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("plain group message", func(t *testing.T) {
		runWebhookPlainMessageCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("multiple commands", func(t *testing.T) {
		runWebhookMultipleCommandsCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("invalid setOff reply", func(t *testing.T) {
		runWebhookInvalidSetOffCase(t, ctx, dynamoClient, sqsClient, resources)
	})
	t.Run("non group update", func(t *testing.T) {
		runWebhookNonGroupCase(t, ctx, dynamoClient, sqsClient, resources)
	})
}

func runWebhookTopTenCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	httpServer := buildIntegrationHTTPServer(t, dynamoClient, sqsClient, resources, integrationServerOptions{})
	response := doHTTP(
		t,
		ctx,
		http.MethodPost,
		httpServer.baseURL+"/webhook",
		webhookTopTenPayload(),
	)
	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Errorf("close HTTP response body: %v", closeErr)
		}
	}()
	assert.Equal(t, http.StatusOK, response.StatusCode)

	assertWebhookChatRow(t, ctx, dynamoClient, resources.chatTable, webhookChatID, webhookChatTitle)
	assertWebhookMessageRow(t, ctx, dynamoClient, resources.messageTable, webhookChatID)

	wantAction, err := command.ActionFor(
		command.Command{Name: "topTen"},
		command.ChatContext{
			ChatID:    webhookChatID,
			ChatTitle: webhookChatTitle,
			UserID:    webhookUserID,
		},
	)
	require.NoError(t, err, "build expected topTen action")
	assertQueuedAction(t, ctx, httpServer, wantAction)
}

func runWebhookPlainMessageCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	const (
		plainChatID    int64 = 42012
		plainChatTitle       = "Plain Webhook"
	)

	httpServer := buildIntegrationHTTPServer(t, dynamoClient, sqsClient, resources, integrationServerOptions{})
	response := doHTTP(
		t,
		ctx,
		http.MethodPost,
		httpServer.baseURL+"/webhook",
		webhookPlainPayload(plainChatID, plainChatTitle),
	)
	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Errorf("close HTTP response body: %v", closeErr)
		}
	}()
	assert.Equal(t, http.StatusOK, response.StatusCode)

	assertWebhookChatRow(t, ctx, dynamoClient, resources.chatTable, plainChatID, plainChatTitle)
	messages, err := appdynamodb.NewMessageClient(dynamoClient).QueryByChat(
		ctx,
		resources.messageTable,
		plainChatID,
		integrationNow.Add(-time.Minute),
	)
	require.NoError(t, err, "query plain webhook message rows")
	require.Len(t, messages, 1)
	assert.Equal(t, plainChatID, messages[0].ChatID)

	assertQueueEmpty(t, ctx, httpServer.queueClient, httpServer.queueURL)
}

func runWebhookMultipleCommandsCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	const (
		multiChatID    int64 = 42013
		multiChatTitle       = "Multi Command Webhook"
		multiUserID    int64 = 10013
	)

	httpServer := buildIntegrationHTTPServer(t, dynamoClient, sqsClient, resources, integrationServerOptions{})
	response := doHTTP(
		t,
		ctx,
		http.MethodPost,
		httpServer.baseURL+"/webhook",
		webhookMultipleCommandsPayload(multiChatID, multiChatTitle, multiUserID),
	)
	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Errorf("close HTTP response body: %v", closeErr)
		}
	}()
	assert.Equal(t, http.StatusOK, response.StatusCode)

	wantActions := []queue.Action{
		mustCommandAction(t, "/jungHelp", multiChatID, multiChatTitle, multiUserID),
		mustCommandAction(t, "/topTen", multiChatID, multiChatTitle, multiUserID),
		mustCommandAction(t, "/allJung", multiChatID, multiChatTitle, multiUserID),
	}
	for _, wantAction := range wantActions {
		assertQueuedAction(t, ctx, httpServer, wantAction)
	}
	assertQueueEmpty(t, ctx, httpServer.queueClient, httpServer.queueURL)
}

func assertWebhookChatRow(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	chatTable string,
	chatID int64,
	chatTitle string,
) {
	t.Helper()

	gotChat, ok, err := appdynamodb.NewChatClient(dynamoClient).Get(ctx, chatTable, chatID)
	require.NoError(t, err, "get webhook chat row")
	require.True(t, ok, "expected webhook chat row")
	assert.Equal(t, chatTitle, gotChat.ChatTitle)
	assert.True(t, gotChat.EnableAllJung)
}

func assertWebhookMessageRow(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	messageTable string,
	chatID int64,
) {
	t.Helper()

	messages, err := appdynamodb.NewMessageClient(dynamoClient).QueryByChat(
		ctx,
		messageTable,
		chatID,
		integrationNow.Add(-time.Minute),
	)
	require.NoError(t, err, "query webhook message rows")
	require.Len(t, messages, 1)
	assert.Equal(t, webhookUserID, messages[0].UserID)
	assert.Equal(t, "floci-user", messages[0].Username)
	assert.Equal(t, "Floci", messages[0].FirstName)
	assert.Equal(t, "Tester", messages[0].LastName)
	assert.Equal(t, message.FormatDateCreated(integrationNow), message.FormatDateCreated(messages[0].DateCreated))
}

func assertQueuedAction(t *testing.T, ctx context.Context, httpServer integrationHTTPServer, wantAction queue.Action) {
	t.Helper()

	queueResponse, err := receiveOne(ctx, httpServer.queueClient, httpServer.queueURL)
	require.NoError(t, err, "receive webhook queue message")

	gotAction := queue.DecodeMessage(queueResponse.Messages[0])
	assertAction(t, wantAction, gotAction)

	err = httpServer.queueClient.Delete(ctx, queue.DeleteMessageRequest{
		QueueURL:      httpServer.queueURL,
		ReceiptHandle: queueResponse.Messages[0].ReceiptHandle,
	})
	require.NoError(t, err, "delete webhook queue message")
}

func assertQueueEmpty(t *testing.T, ctx context.Context, queueClient queueClient, queueURL string) {
	t.Helper()

	response, err := queueClient.ReceiveMessage(ctx, queue.ReceiveMessageRequest{
		QueueURL:            queueURL,
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     1,
	})
	require.NoError(t, err, "poll empty queue")
	assert.Empty(t, response.Messages)
}

func mustCommandAction(
	t *testing.T,
	text string,
	chatID int64,
	chatTitle string,
	userID int64,
) queue.Action {
	t.Helper()

	commands := command.ParseAll(text)
	require.Len(t, commands, 1, text)

	action, err := command.ActionFor(commands[0], command.ChatContext{
		ChatID:    chatID,
		ChatTitle: chatTitle,
		UserID:    userID,
	})
	require.NoError(t, err, "build %s action", text)

	return action
}

func runWebhookInvalidSetOffCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	const (
		invalidChatID    int64 = 42015
		invalidChatTitle       = "Invalid SetOff Webhook"
		invalidUserID    int64 = 10015
	)

	messenger := &recordingMessenger{}
	httpServer := buildIntegrationHTTPServer(t, dynamoClient, sqsClient, resources, integrationServerOptions{
		messenger: messenger,
	})
	response := doHTTP(
		t,
		ctx,
		http.MethodPost,
		httpServer.baseURL+"/webhook",
		webhookCommandPayload(invalidChatID, invalidChatTitle, invalidUserID, "/setOffFromWorkTimeUTC bad"),
	)
	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Errorf("close HTTP response body: %v", closeErr)
		}
	}()
	assert.Equal(t, http.StatusOK, response.StatusCode)

	messages := messenger.recordedMessages()
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].text, "Error: Invalid format for setOffFromWorkTimeUTC")
	assertQueueEmpty(t, ctx, httpServer.queueClient, httpServer.queueURL)
}

func runWebhookNonGroupCase(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	httpServer := buildIntegrationHTTPServer(t, dynamoClient, sqsClient, resources, integrationServerOptions{})
	response := doHTTP(
		t,
		ctx,
		http.MethodPost,
		httpServer.baseURL+"/webhook",
		`{"message":{"chat":{"id":42016,"title":"Private","type":"private"},"text":"hello"}}`,
	)
	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Errorf("close HTTP response body: %v", closeErr)
		}
	}()

	assert.Equal(t, http.StatusNoContent, response.StatusCode)
	assertQueueEmpty(t, ctx, httpServer.queueClient, httpServer.queueURL)

	_, ok, err := appdynamodb.NewChatClient(dynamoClient).Get(ctx, resources.chatTable, 42016)
	require.NoError(t, err, "get chat after non-group webhook")
	assert.False(t, ok, "non-group webhook should not persist chat metadata")
}

func webhookTopTenPayload() string {
	return webhookCommandPayload(webhookChatID, webhookChatTitle, webhookUserID, "/topTen")
}

func webhookPlainPayload(chatID int64, chatTitle string) string {
	return `{"message":{"chat":{"id":` + formatInt(chatID) + `,"title":"` + chatTitle + `","type":"group"},"text":"hello","entities":[]}}`
}

func webhookMultipleCommandsPayload(chatID int64, chatTitle string, userID int64) string {
	return webhookCommandPayload(chatID, chatTitle, userID, "/allJung /topTen /jungHelp")
}

func webhookCommandPayload(chatID int64, chatTitle string, userID int64, text string) string {
	return `{"message":{"chat":{"id":` + formatInt(chatID) + `,"title":"` + chatTitle + `","type":"supergroup"},"from":{"id":` + formatInt(userID) + `,"username":"floci-user","first_name":"Floci","last_name":"Tester"},"text":"` + text + `","entities":[{"type":"bot_command"}]}}`
}

func formatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
