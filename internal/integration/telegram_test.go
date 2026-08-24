package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

type telegramTestHarness struct {
	server   *httptest.Server
	messages []telegramCapturedMessage
	mutex    sync.Mutex
}

type telegramCapturedMessage struct {
	chatID int64
	text   string
}

func newTelegramTestHarness(t *testing.T, adminUserID int64) (*telegramTestHarness, telegram.Client) {
	t.Helper()

	harness := &telegramTestHarness{}
	harness.server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/sendMessage"):
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("read Telegram sendMessage body: %v", err)
				response.WriteHeader(http.StatusInternalServerError)
				return
			}

			var payload struct {
				ChatID int64  `json:"chat_id"`
				Text   string `json:"text"`
			}
			err = json.Unmarshal(body, &payload)
			if err != nil {
				t.Errorf("decode Telegram sendMessage body: %v", err)
				response.WriteHeader(http.StatusInternalServerError)
				return
			}

			harness.mutex.Lock()
			harness.messages = append(harness.messages, telegramCapturedMessage{
				chatID: payload.ChatID,
				text:   payload.Text,
			})
			harness.mutex.Unlock()

			response.WriteHeader(http.StatusOK)
			_, writeErr := response.Write([]byte(`{"ok":true}`))
			if writeErr != nil {
				t.Errorf("write Telegram sendMessage response: %v", writeErr)
			}
		case strings.HasSuffix(request.URL.Path, "/getChatAdministrators"):
			response.WriteHeader(http.StatusOK)
			_, writeErr := response.Write([]byte(`{"ok":true,"result":[{"user":{"id":` + formatInt(adminUserID) + `}}]}`))
			if writeErr != nil {
				t.Errorf("write Telegram getChatAdministrators response: %v", writeErr)
			}
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(harness.server.Close)

	client := telegram.NewClient(
		"integration-token",
		telegram.WithBaseURL(harness.server.URL),
		telegram.WithHTTPClient(harness.server.Client()),
	)

	return harness, client
}

func (harness *telegramTestHarness) capturedMessages() []telegramCapturedMessage {
	harness.mutex.Lock()
	defer harness.mutex.Unlock()

	copied := make([]telegramCapturedMessage, len(harness.messages))
	copy(copied, harness.messages)

	return copied
}

func runWebhookTelegramClientIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	const (
		telegramChatID    int64 = 42020
		telegramChatTitle       = "Telegram Client Webhook"
		telegramUserID    int64 = 10020
	)

	harness, telegramClient := newTelegramTestHarness(t, telegramUserID)
	httpServer := buildIntegrationHTTPServer(t, dynamoClient, sqsClient, resources, integrationServerOptions{
		messenger: telegramClient,
	})

	response := doHTTP(
		t,
		ctx,
		http.MethodPost,
		httpServer.baseURL+"/webhook",
		webhookCommandPayload(telegramChatID, telegramChatTitle, telegramUserID, "/setOffFromWorkTimeUTC bad"),
	)
	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Errorf("close HTTP response body: %v", closeErr)
		}
	}()
	assert.Equal(t, http.StatusOK, response.StatusCode)

	messages := harness.capturedMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, telegramChatID, messages[0].chatID)
	assert.Contains(t, messages[0].text, "Error: Invalid format for setOffFromWorkTimeUTC")
	assertQueueEmpty(t, ctx, httpServer.queueClient, httpServer.queueURL)
}
