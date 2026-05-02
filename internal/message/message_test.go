package message

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositorySaveBuildsContractUpdate(t *testing.T) {
	client := &fakeMessageClient{}
	repository := Repository{TableName: "messages-dev", Client: client}
	stored := Message{ChatID: 123, DateCreated: mustParseTime(t, "2019-04-01T02:38:24Z"), TTL: 1554691104}

	err := repository.Save(context.Background(), stored)

	require.NoError(t, err)
	require.Len(t, client.updates, 1)
	assert.Equal(t, BuildSaveUpdate("messages-dev", stored), client.updates[0])
}

func TestRepositorySaveDefaultsTimeAndTTL(t *testing.T) {
	client := &fakeMessageClient{}
	now := mustParseTime(t, "2019-04-01T02:38:24Z")
	repository := Repository{
		TableName: "messages-dev",
		Client:    client,
		Now: func() time.Time {
			return now
		},
	}

	err := repository.Save(context.Background(), Message{ChatID: 123})

	require.NoError(t, err)
	assert.Equal(t, "2019-04-01T10:38:24+08:00", client.updates[0].Key["dateCreated"])
	assert.Equal(t, int64(1554691104), client.updates[0].ExpressionAttributeValues[":ttl"])
}

func TestRepositorySaveRequiresClient(t *testing.T) {
	err := (Repository{}).Save(context.Background(), Message{})

	require.Error(t, err)
	assert.EqualError(t, err, "message repository client is required")
}

func TestRepositorySaveWrapsClientError(t *testing.T) {
	err := (Repository{Client: &fakeMessageClient{err: errors.New("boom")}}).Save(context.Background(), Message{})

	require.Error(t, err)
	assert.EqualError(t, err, "save message: boom")
}

func TestRepositoryQueryByChatBuildsRequest(t *testing.T) {
	rows := []Message{{ChatID: 123}}
	client := &fakeMessageClient{rows: rows}
	since := mustParseTime(t, "2019-04-01T00:00:00Z")
	until := mustParseTime(t, "2019-04-02T00:00:00Z")

	got, err := (Repository{TableName: "messages-dev", Client: client}).QueryByChat(context.Background(), 123, since, until)

	require.NoError(t, err)
	assert.Equal(t, rows, got)
	assert.Equal(t, QueryRequest{TableName: "messages-dev", ChatID: 123, Since: since, Until: until, Descending: true}, client.query)
}

func TestRepositoryQueryByChatRequiresClient(t *testing.T) {
	_, err := (Repository{}).QueryByChat(context.Background(), 123, time.Time{}, time.Time{})

	require.Error(t, err)
	assert.EqualError(t, err, "message repository client is required")
}

func TestRepositoryQueryByChatWrapsClientError(t *testing.T) {
	_, err := (Repository{Client: &fakeMessageClient{err: errors.New("boom")}}).QueryByChat(context.Background(), 123, time.Time{}, time.Time{})

	require.Error(t, err)
	assert.EqualError(t, err, "query messages by chat: boom")
}

func TestFromTelegramBuildsStoredMessage(t *testing.T) {
	now := mustParseTime(t, "2019-04-01T02:38:24Z")

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
	now := mustParseTime(t, "2019-04-01T02:38:24Z")

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
	now := mustParseTime(t, "2019-04-01T02:38:24Z")

	assert.Equal(t, int64(1554691104), TTL(now, DefaultTTL))
}

func TestBuildSaveUpdatePreservesContractDynamoDBShape(t *testing.T) {
	stored := Message{
		ChatID:      123,
		DateCreated: mustParseTime(t, "2019-04-01T02:38:24Z"),
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
	assert.Equal(t, "SET #ttl = :ttl, #chatTitle = :chatTitle, #userId = :userId, #username = :username, #firstName = :firstName, #lastName = :lastName", update.UpdateExpression)
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
		DateCreated: mustParseTime(t, "2019-04-01T02:38:24Z"),
		TTL:         1554691104,
	}

	update := BuildSaveUpdate("messages-dev", stored)

	assert.Equal(t, "SET #ttl = :ttl", update.UpdateExpression)
	assert.Equal(t, map[string]string{"#ttl": "ttl"}, update.ExpressionAttributeNames)
	assert.Equal(t, map[string]any{":ttl": int64(1554691104)}, update.ExpressionAttributeValues)
}

func TestJoinAssignmentsHandlesSingleAssignment(t *testing.T) {
	assert.Equal(t, "#ttl = :ttl", joinAssignments([]string{"#ttl = :ttl"}))
}

func mustParseTime(t *testing.T, raw string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err)

	return parsed
}

type fakeMessageClient struct {
	updates []UpdateExpression
	query   QueryRequest
	rows    []Message
	err     error
}

func (client *fakeMessageClient) Update(ctx context.Context, request UpdateExpression) error {
	client.updates = append(client.updates, request)
	return client.err
}

func (client *fakeMessageClient) QueryByChat(ctx context.Context, request QueryRequest) ([]Message, error) {
	client.query = request
	return client.rows, client.err
}
