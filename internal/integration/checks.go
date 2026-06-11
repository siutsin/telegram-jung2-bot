package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/command"
	appdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

const (
	integrationChatID    = 42001
	integrationChatTitle = "Floci Integration"
	integrationUserID    = 10001
)

type queueActionCase struct {
	name   string
	action queue.Action
}

type commandActionSpec struct {
	name string
	text string
}

func runDynamoDBIntegration(t *testing.T, ctx context.Context, client *awsdynamodb.Client, resources testResources) {
	t.Helper()

	chatRepo := appdynamodb.NewChatClient(client)
	messageRepo := appdynamodb.NewMessageClient(client)

	created := time.Date(2026, 6, 11, 18, 30, 0, 0, time.UTC)
	settings := chat.ChatSetting{
		ChatID:      integrationChatID,
		ChatTitle:   integrationChatTitle,
		DateCreated: created,
		TTL:         message.TTL(created, message.DefaultTTL),
	}
	err := chatRepo.Save(ctx, resources.chatTable, settings)
	require.NoError(t, err, "save chat metadata")

	gotChat, ok, err := chatRepo.Get(ctx, resources.chatTable, settings.ChatID)
	require.NoError(t, err, "get chat metadata")
	require.True(t, ok, "expected stored chat metadata")
	assert.Equal(t, settings.ChatTitle, gotChat.ChatTitle)
	assert.True(t, gotChat.EnableAllJung)

	err = chatRepo.Update(ctx, chat.BuildOffWorkUpdate(resources.chatTable, settings.ChatID, "1830", workday.Thu))
	require.NoError(t, err, "save chat off-work settings")
	due, err := chatRepo.DueChatIDs(ctx, resources.chatTable, created)
	require.NoError(t, err, "scan due chats")
	assert.Equal(t, []int64{settings.ChatID}, due)

	row := message.Message{
		ChatID:      settings.ChatID,
		DateCreated: created,
		ChatTitle:   settings.ChatTitle,
		UserID:      integrationUserID,
		Username:    "floci-user",
		FirstName:   "Floci",
		LastName:    "Tester",
		TTL:         message.TTL(created, message.DefaultTTL),
	}
	err = messageRepo.Save(ctx, resources.messageTable, row)
	require.NoError(t, err, "save message row")

	messages, err := messageRepo.QueryByChat(ctx, resources.messageTable, settings.ChatID, created.Add(-time.Minute))
	require.NoError(t, err, "query messages by chat")
	require.Len(t, messages, 1)
	assert.Equal(t, row.Username, messages[0].Username)
	assert.Equal(t, row.UserID, messages[0].UserID)
	assert.True(t, messages[0].DateCreated.Equal(row.DateCreated), "dateCreated")

	runDynamoDBMultiMessageQuery(t, ctx, messageRepo, resources.messageTable, settings.ChatID, created)
	runDynamoDBListEnabled(t, ctx, chatRepo, resources.chatTable, settings)
}

func runDynamoDBMultiMessageQuery(
	t *testing.T,
	ctx context.Context,
	messageRepo appdynamodb.MessageClient,
	messageTable string,
	chatID int64,
	baseTime time.Time,
) {
	t.Helper()

	extraUsers := []struct {
		username string
		offset   time.Duration
	}{
		{username: "second-user", offset: 2 * time.Minute},
		{username: "third-user", offset: 3 * time.Minute},
	}
	for _, user := range extraUsers {
		row := message.Message{
			ChatID:      chatID,
			DateCreated: baseTime.Add(user.offset),
			ChatTitle:   integrationChatTitle,
			UserID:      integrationUserID,
			Username:    user.username,
			TTL:         message.TTL(baseTime, message.DefaultTTL),
		}
		err := messageRepo.Save(ctx, messageTable, row)
		require.NoError(t, err, "save extra message row for %s", user.username)
	}

	messages, err := messageRepo.QueryByChat(ctx, messageTable, chatID, baseTime.Add(-time.Minute))
	require.NoError(t, err, "query multiple messages by chat")
	require.Len(t, messages, 3)
	assert.Equal(t, "third-user", messages[0].Username)
	assert.Equal(t, "second-user", messages[1].Username)
	assert.Equal(t, "floci-user", messages[2].Username)
}

func runDynamoDBListEnabled(
	t *testing.T,
	ctx context.Context,
	chatRepo appdynamodb.ChatClient,
	chatTable string,
	primary chat.ChatSetting,
) {
	t.Helper()

	const (
		secondChatID    int64 = 42011
		secondChatTitle       = "Second Floci Chat"
	)

	second := chat.ChatSetting{
		ChatID:        secondChatID,
		ChatTitle:     secondChatTitle,
		DateCreated:   primary.DateCreated,
		TTL:           primary.TTL,
		EnableAllJung: true,
	}
	err := chatRepo.Save(ctx, chatTable, second)
	require.NoError(t, err, "save second enabled chat")

	rows, err := chatRepo.ListEnabled(ctx, chatTable)
	require.NoError(t, err, "list enabled chats")
	require.Len(t, rows, 2)

	chatIDs := []int64{rows[0].ChatID, rows[1].ChatID}
	assert.Contains(t, chatIDs, primary.ChatID)
	assert.Contains(t, chatIDs, secondChatID)
}

func runSQSIntegration(t *testing.T, ctx context.Context, client *awssqs.Client, resources testResources) {
	t.Helper()

	queueClient := queue.NewClient(client)
	producer := queue.NewProducer(resources.queueURL, queueClient)

	actionCases := queueActionCases(t)

	t.Run("Lambda attribute casing", func(t *testing.T) {
		runSQSAttributeCasingIntegration(t)
	})

	for _, testCase := range actionCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := producer.Enqueue(ctx, testCase.action)
			require.NoError(t, err, "enqueue %s action", testCase.action.Name)

			response, receiveErr := receiveOne(ctx, queueClient, resources.queueURL)
			require.NoError(t, receiveErr)

			got := queue.DecodeMessage(response.Messages[0])
			assertAction(t, testCase.action, got)

			err = queueClient.Delete(ctx, queue.DeleteMessageRequest{
				QueueURL:      resources.queueURL,
				ReceiptHandle: response.Messages[0].ReceiptHandle,
			})
			require.NoError(t, err, "delete %s queue message", testCase.action.Name)
		})
	}
}

func queueActionCases(t *testing.T) []queueActionCase {
	t.Helper()

	chatContext := command.ChatContext{
		ChatID:    integrationChatID,
		ChatTitle: integrationChatTitle,
		UserID:    integrationUserID,
	}

	commandSpecs := []commandActionSpec{
		{name: "Telegram /jungHelp", text: "/jungHelp"},
		{name: "Telegram /topTen", text: "/topTen"},
		{name: "Telegram /topDiver", text: "/topDiver"},
		{name: "Telegram /allJung", text: "/allJung"},
		{name: "Telegram /enableAllJung", text: "/enableAllJung"},
		{name: "Telegram /disableAllJung", text: "/disableAllJung"},
		{name: "Telegram /setOffFromWorkTimeUTC", text: "/setOffFromWorkTimeUTC 1830 MON,TUE"},
	}

	actionCases := make([]queueActionCase, 0, len(commandSpecs)+2)
	for _, spec := range commandSpecs {
		testCase := buildCommandActionCase(t, spec, chatContext)
		actionCases = append(actionCases, testCase)
	}

	actionCases = append(actionCases,
		queueActionCase{
			name:   "Scheduler onOffFromWork",
			action: schedule.BuildOnOffFromWorkAction("2026-06-11T18:30:00Z"),
		},
		queueActionCase{
			name:   "Scheduler offFromWork",
			action: schedule.BuildOffFromWorkAction(chatContext.ChatID),
		},
	)

	return actionCases
}

func buildCommandActionCase(t *testing.T, spec commandActionSpec, chatContext command.ChatContext) queueActionCase {
	t.Helper()

	commands := command.ParseAll(spec.text)
	require.Len(t, commands, 1, spec.name)

	action, err := command.ActionFor(commands[0], chatContext)
	require.NoError(t, err, "%s: build action", spec.name)

	return queueActionCase{name: spec.name, action: action}
}

func assertAction(t *testing.T, want queue.Action, got queue.Action) {
	t.Helper()

	assert.Equal(t, want.Name, got.Name)
	assert.Equal(t, want.Body, got.Body)
	assert.Equal(t, want.Attributes, got.Attributes)
}

func receiveOne(ctx context.Context, client interface {
	ReceiveMessage(context.Context, queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error)
}, queueURL string) (queue.ReceiveMessageResponse, error) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		response, err := client.ReceiveMessage(ctx, queue.ReceiveMessageRequest{
			QueueURL:            queueURL,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     1,
		})
		if err != nil {
			return queue.ReceiveMessageResponse{}, fmt.Errorf("receive queue message: %w", err)
		}
		if len(response.Messages) > 0 {
			return response, nil
		}
		if time.Now().After(deadline) {
			return queue.ReceiveMessageResponse{}, errors.New("timed out waiting for SQS message")
		}
	}
}

func runSQSAttributeCasingIntegration(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		raw  string
		want queue.Action
	}{
		{
			name: "lower case action attribute",
			raw:  `{"body":"sendTopTenMessage","messageAttributes":{"action":{"stringValue":"topten"},"chatId":{"stringValue":"42001"}}}`,
			want: queue.Action{
				Name: queue.ActionTopTen,
				Body: queue.BodyTopTen,
				Attributes: map[string]string{
					"action": queue.ActionTopTen,
					"chatId": "42001",
				},
			},
		},
		{
			name: "lower case wins over upper case",
			raw:  `{"body":"sendAllJungMessage","messageAttributes":{"action":{"StringValue":"topten","stringValue":"alljung"}}}`,
			want: queue.Action{
				Name: queue.ActionAllJung,
				Body: queue.BodyAllJung,
				Attributes: map[string]string{
					"action": queue.ActionAllJung,
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var raw queue.RawMessage
			err := json.Unmarshal([]byte(testCase.raw), &raw)
			require.NoError(t, err, "unmarshal raw SQS message")

			got := queue.DecodeMessage(raw)
			assertAction(t, testCase.want, got)
		})
	}
}
