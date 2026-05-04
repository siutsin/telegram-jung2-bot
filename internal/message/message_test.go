package message

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

func TestFromTelegramBuildsStoredMessage(t *testing.T) {
	now := mustParseTime(t)

	stored := FromTelegram(telegram.Message{
		Chat: telegram.Chat{ID: 123, Title: "title"},
		From: &telegram.User{ID: 234, UserName: "username", FirstName: "first_name", LastName: "last_name"},
	}, now)

	assert.Equal(t, int64(123), stored.ChatID)
	assert.Equal(t, "title", stored.ChatTitle)
	assert.Equal(t, int64(234), stored.UserID)
	assert.Equal(t, "username", stored.Username)
	assert.Equal(t, "first_name", stored.FirstName)
	assert.Equal(t, "last_name", stored.LastName)
	assert.Equal(t, "2019-04-01T10:38:24+08:00", FormatDateCreated(stored.DateCreated))
	assert.Equal(t, now.Add(DefaultTTL).Unix(), stored.TTL)
}

func TestFromTelegramHandlesMissingOptionalUser(t *testing.T) {
	now := mustParseTime(t)

	stored := FromTelegram(telegram.Message{Chat: telegram.Chat{ID: 123}}, now)

	assert.Equal(t, int64(123), stored.ChatID)
	assert.Empty(t, stored.ChatTitle)
	assert.Zero(t, stored.UserID)
	assert.Empty(t, stored.Username)
	assert.Empty(t, stored.FirstName)
	assert.Empty(t, stored.LastName)
}

func TestDateCreatedContractFormatRoundTrip(t *testing.T) {
	parsed, err := ParseDateCreated("2019-03-16T02:26:19+08:00")
	require.NoError(t, err)

	assert.Equal(t, "2019-03-16T02:26:19+08:00", FormatDateCreated(parsed))
}

func TestParseDateCreatedRejectsInvalidValue(t *testing.T) {
	_, err := ParseDateCreated("not-a-date")

	require.Error(t, err)
}

func TestTTLUsesContractSevenDayRetention(t *testing.T) {
	now := mustParseTime(t)

	assert.Equal(t, int64(1554691104), TTL(now, DefaultTTL))
}

func TestBuildSaveUpdatePreservesContractDynamoDBShape(t *testing.T) {
	stored := Message{
		ChatID:      123,
		DateCreated: mustParseTime(t),
		ChatTitle:   "title",
		UserID:      234,
		Username:    "username",
		FirstName:   "first_name",
		LastName:    "last_name",
		TTL:         1554691104,
	}

	update := BuildSaveUpdate("messages-dev", stored)

	assert.Equal(t, "messages-dev", update.TableName)
	assert.Equal(t, map[string]any{
		"chatId":      int64(123),
		"dateCreated": "2019-04-01T10:38:24+08:00",
	}, update.Key)
	assert.Equal(t, "SET #chatTitle = :chatTitle, #userId = :userId, #username = :username, #firstName = :firstName, #lastName = :lastName, #ttl = :ttl", update.UpdateExpression)
	assert.Equal(t, map[string]string{
		"#ttl":       "ttl",
		"#chatTitle": "chatTitle",
		"#userId":    "userId",
		"#username":  "username",
		"#firstName": "firstName",
		"#lastName":  "lastName",
	}, update.ExpressionAttributeNames)
	assert.Equal(t, map[string]any{
		":ttl":       int64(1554691104),
		":chatTitle": "title",
		":userId":    int64(234),
		":username":  "username",
		":firstName": "first_name",
		":lastName":  "last_name",
	}, update.ExpressionAttributeValues)
}

func TestBuildSaveUpdateOmitsMissingOptionalAttributes(t *testing.T) {
	stored := Message{
		ChatID:      123,
		DateCreated: mustParseTime(t),
		TTL:         1554691104,
	}

	update := BuildSaveUpdate("messages-dev", stored)

	assert.Equal(t, "SET #ttl = :ttl", update.UpdateExpression)
	assert.Equal(t, map[string]string{"#ttl": "ttl"}, update.ExpressionAttributeNames)
	assert.Equal(t, map[string]any{":ttl": int64(1554691104)}, update.ExpressionAttributeValues)
}

func TestIsZeroAttributeValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "empty string", value: "", want: true},
		{name: "string", value: "value", want: false},
		{name: "zero int64", value: int64(0), want: true},
		{name: "int64", value: int64(1), want: false},
		{name: "nil", value: nil, want: true},
		{name: "unknown", value: true, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isZeroAttributeValue(tc.value))
		})
	}
}

func mustParseTime(t *testing.T) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, "2019-04-01T02:38:24Z")
	require.NoError(t, err)

	return parsed
}
