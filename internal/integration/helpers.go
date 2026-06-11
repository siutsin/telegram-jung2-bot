package integration

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/require"

	appdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

const integrationStage = "dev"

var integrationNow = time.Date(2026, 6, 11, 18, 30, 0, 0, time.UTC)

type recordingMessenger struct {
	mutex    sync.Mutex
	messages []recordedMessage
	admin    bool
}

type recordedMessage struct {
	chatID  int64
	text    string
	options telegram.SendMessageOptions
}

func (messenger *recordingMessenger) SendMessage(_ context.Context, chatID int64, text string) error {
	messenger.mutex.Lock()
	defer messenger.mutex.Unlock()
	messenger.messages = append(messenger.messages, recordedMessage{chatID: chatID, text: text})
	return nil
}

func (messenger *recordingMessenger) SendMessageWithOptions(
	_ context.Context,
	chatID int64,
	text string,
	options telegram.SendMessageOptions,
) error {
	messenger.mutex.Lock()
	defer messenger.mutex.Unlock()
	messenger.messages = append(messenger.messages, recordedMessage{
		chatID:  chatID,
		text:    text,
		options: options,
	})
	return nil
}

func (messenger *recordingMessenger) recordedMessages() []recordedMessage {
	messenger.mutex.Lock()
	defer messenger.mutex.Unlock()

	copied := make([]recordedMessage, len(messenger.messages))
	copy(copied, messenger.messages)

	return copied
}

func (messenger *recordingMessenger) IsAdmin(_ context.Context, _ int64, _ int64) (bool, error) {
	return messenger.admin, nil
}

type queueClient interface {
	ReceiveMessage(context.Context, queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error)
	Delete(context.Context, queue.DeleteMessageRequest) error
}

type integrationHTTPServer struct {
	baseURL     string
	queueURL    string
	queueClient queueClient
}

type integrationServerOptions struct {
	stage     string
	messenger interface {
		SendMessage(context.Context, int64, string) error
	}
	scaleUpper appdynamodb.ScaleUpper
}

func buildIntegrationHTTPServer(
	t *testing.T,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
	options integrationServerOptions,
) integrationHTTPServer {
	t.Helper()

	queueClient := queue.NewClient(sqsClient)
	producer := queue.NewProducer(resources.queueURL, queueClient)

	messenger := options.messenger
	if messenger == nil {
		messenger = noopMessenger{}
	}

	deps := httpserver.Dependencies{
		ChatTable:    resources.chatTable,
		MessageTable: resources.messageTable,
		Messages:     appdynamodb.NewMessageClient(dynamoClient),
		Chats:        appdynamodb.NewChatClient(dynamoClient),
		Enqueuer:     producer,
		Messenger:    messenger,
		ScaleUpper:   options.scaleUpper,
		Now: func() time.Time {
			return integrationNow
		},
	}

	server, err := httpserver.NewServer(":0", 5*time.Second, options.stage, deps)
	require.NoError(t, err, "create HTTP server")

	testServer := httptest.NewServer(server.Handler)
	t.Cleanup(testServer.Close)

	return integrationHTTPServer{
		baseURL:     testServer.URL,
		queueURL:    resources.queueURL,
		queueClient: queueClient,
	}
}

func doHTTP(
	t *testing.T,
	ctx context.Context,
	method string,
	url string,
	body string,
) *http.Response {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	require.NoError(t, err, "build HTTP request")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err, "send HTTP request")

	return response
}

func readResponseBody(t *testing.T, response *http.Response) string {
	t.Helper()

	payload, err := io.ReadAll(response.Body)
	require.NoError(t, err, "read HTTP response body")

	return string(payload)
}

func drainQueue(t *testing.T, ctx context.Context, queueClient queueClient, queueURL string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := queueClient.ReceiveMessage(ctx, queue.ReceiveMessageRequest{
			QueueURL:            queueURL,
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     1,
		})
		require.NoError(t, err, "drain queue")
		if len(response.Messages) == 0 {
			return
		}
		for _, item := range response.Messages {
			err = queueClient.Delete(ctx, queue.DeleteMessageRequest{
				QueueURL:      queueURL,
				ReceiptHandle: item.ReceiptHandle,
			})
			require.NoError(t, err, "delete drained queue message")
		}
	}
}
