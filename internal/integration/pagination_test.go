package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	bot "github.com/siutsin/telegram-jung2-bot/internal"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
)

const (
	dueChatPaginationCount        = 120
	dueChatPaginationBaseID int64 = 51000
	messagePaginationCount        = 120
	messagePaginationChatID int64 = 52000
)

func runDynamoDBDueChatPaginationIntegration(
	t *testing.T,
	ctx context.Context,
	client *awsdynamodb.Client,
	resources testResources,
) {
	t.Helper()

	chatRepo := appdynamodb.NewChatClient(client)
	created := integrationNow

	for index := range dueChatPaginationCount {
		chatID := dueChatPaginationBaseID + int64(index)
		settings := bot.ChatSetting{
			ChatID:        chatID,
			ChatTitle:     fmt.Sprintf("Due Pagination Chat %d", index),
			DateCreated:   created,
			TTL:           bot.TTL(created, bot.DefaultTTL),
			EnableAllJung: true,
		}
		err := chatRepo.Save(ctx, resources.chatTable, settings)
		require.NoError(t, err, "seed due pagination chat %d", index)
		err = chatRepo.Update(ctx, bot.BuildOffWorkUpdate(resources.chatTable, chatID, "1830", bot.Thu))
		require.NoError(t, err, "seed due pagination off-work settings %d", index)
	}

	due, err := chatRepo.DueChatIDs(ctx, resources.chatTable, created)
	require.NoError(t, err, "scan due chats across pages")
	require.Len(t, due, dueChatPaginationCount)

	dueSet := make(map[int64]struct{}, len(due))
	for _, chatID := range due {
		dueSet[chatID] = struct{}{}
	}
	for index := range dueChatPaginationCount {
		_, ok := dueSet[dueChatPaginationBaseID+int64(index)]
		assert.True(t, ok, "expected due pagination chat %d", index)
	}
}

func runDynamoDBMessageQueryPaginationIntegration(
	t *testing.T,
	ctx context.Context,
	client *awsdynamodb.Client,
	resources testResources,
) {
	t.Helper()

	messageRepo := appdynamodb.NewMessageClient(client)
	baseTime := integrationNow.Add(-time.Hour)

	for index := range messagePaginationCount {
		row := bot.StoredMessage{
			ChatID:      messagePaginationChatID,
			DateCreated: baseTime.Add(time.Duration(index) * time.Minute),
			ChatTitle:   "Message Pagination",
			UserID:      integrationUserID,
			Username:    fmt.Sprintf("user-%d", index),
			TTL:         bot.TTL(baseTime, bot.DefaultTTL),
		}
		err := messageRepo.Save(ctx, resources.messageTable, row)
		require.NoError(t, err, "seed pagination message %d", index)
	}

	messages, err := messageRepo.QueryByChat(
		ctx,
		resources.messageTable,
		messagePaginationChatID,
		baseTime.Add(-time.Minute),
	)
	require.NoError(t, err, "query messages across pages")
	require.Len(t, messages, messagePaginationCount)
	assert.Equal(t, fmt.Sprintf("user-%d", messagePaginationCount-1), messages[0].Username)
	assert.Equal(t, "user-0", messages[len(messages)-1].Username)
}
