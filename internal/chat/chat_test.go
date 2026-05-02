package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryGetReturnsStoredSettings(t *testing.T) {
	enabled := false
	client := &fakeChatClient{row: Row{ChatID: 123, ChatTitle: "title", EnableAllJung: &enabled}}

	settings, err := (Repository{TableName: "chat-id-dev", Client: client}).Get(context.Background(), 123)

	require.NoError(t, err)
	assert.Equal(t, int64(123), settings.ChatID)
	assert.Equal(t, "title", settings.ChatTitle)
	assert.False(t, settings.EnableAllJung)
	assert.Equal(t, "chat-id-dev", client.getTable)
	assert.Equal(t, int64(123), client.getChatID)
}

func TestRepositoryGetDefaultsMissingRow(t *testing.T) {
	settings, err := (Repository{Client: &fakeChatClient{missing: true}}).Get(context.Background(), 123)

	require.NoError(t, err)
	assert.Equal(t, int64(123), settings.ChatID)
	assert.True(t, settings.EnableAllJung)
}

func TestRepositoryGetRequiresClient(t *testing.T) {
	_, err := (Repository{}).Get(context.Background(), 123)

	require.Error(t, err)
	assert.EqualError(t, err, "chat repository client is required")
}

func TestRepositoryGetWrapsClientError(t *testing.T) {
	_, err := (Repository{Client: &fakeChatClient{err: errors.New("boom")}}).Get(context.Background(), 123)

	require.Error(t, err)
	assert.EqualError(t, err, "get chat settings: boom")
}

func TestRepositoryGetWrapsParseError(t *testing.T) {
	_, err := (Repository{Client: &fakeChatClient{row: Row{DateCreated: "bad"}}}).Get(context.Background(), 123)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse chat settings")
}

func TestRepositorySaveBuildsMetadataUpdate(t *testing.T) {
	client := &fakeChatClient{}
	settings := Settings{ChatID: 123, ChatTitle: "title", DateCreated: mustParseTime(t, "2019-04-01T02:38:24Z"), TTL: 1554691104}

	err := (Repository{TableName: "chat-id-dev", Client: client}).Save(context.Background(), settings)

	require.NoError(t, err)
	assert.Equal(t, []UpdateExpression{BuildMetadataUpdate("chat-id-dev", settings)}, client.updates)
}

func TestRepositorySaveRequiresClient(t *testing.T) {
	err := (Repository{}).Save(context.Background(), Settings{})

	require.Error(t, err)
	assert.EqualError(t, err, "chat repository client is required")
}

func TestRepositorySaveWrapsClientError(t *testing.T) {
	err := (Repository{Client: &fakeChatClient{err: errors.New("boom")}}).Save(context.Background(), Settings{})

	require.Error(t, err)
	assert.EqualError(t, err, "save chat settings: boom")
}

func TestRepositoryListEnabledReturnsDisabledAllJungRowsForScheduleParity(t *testing.T) {
	disabled := false
	client := &fakeChatClient{rows: []Row{
		{ChatID: 1},
		{ChatID: 2, EnableAllJung: &disabled},
	}}

	settings, err := (Repository{TableName: "chat-id-dev", Client: client}).ListEnabled(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []Settings{{ChatID: 1, EnableAllJung: true}, {ChatID: 2, EnableAllJung: false}}, settings)
	assert.Equal(t, "chat-id-dev", client.listTable)
}

func TestRepositoryListEnabledRequiresClient(t *testing.T) {
	_, err := (Repository{}).ListEnabled(context.Background())

	require.Error(t, err)
	assert.EqualError(t, err, "chat repository client is required")
}

func TestRepositoryListEnabledWrapsClientError(t *testing.T) {
	_, err := (Repository{Client: &fakeChatClient{err: errors.New("boom")}}).ListEnabled(context.Background())

	require.Error(t, err)
	assert.EqualError(t, err, "list enabled chats: boom")
}

func TestRepositoryListEnabledIgnoresMalformedDateCreated(t *testing.T) {
	settings, err := (Repository{Client: &fakeChatClient{rows: []Row{{ChatID: 123, DateCreated: "bad"}}}}).ListEnabled(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []Settings{{ChatID: 123, EnableAllJung: true}}, settings)
}

func TestRepositoryListEnabledMasksMalformedWorkdayBits(t *testing.T) {
	mask := workday.Mon | 128

	settings, err := (Repository{Client: &fakeChatClient{rows: []Row{{ChatID: 123, Workday: &mask}}}}).ListEnabled(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []Settings{{ChatID: 123, EnableAllJung: true, Workday: workday.Workdays(workday.Mon), HasWorkday: true}}, settings)
}

func TestFromTelegramBuildsChatMetadata(t *testing.T) {
	now := mustParseTime(t, "2019-04-01T02:38:24Z")

	settings := FromTelegram(telegram.Message{Chat: telegram.Chat{ID: 123, Title: "title"}}, now)

	assert.Equal(t, int64(123), settings.ChatID)
	assert.Equal(t, "title", settings.ChatTitle)
	assert.Equal(t, now, settings.DateCreated)
	assert.Equal(t, int64(1554691104), settings.TTL)
	assert.True(t, settings.EnableAllJung)
}

func TestFromRowAppliesContractDefaults(t *testing.T) {
	settings, err := FromRow(Row{ChatID: 123})
	require.NoError(t, err)

	assert.Equal(t, int64(123), settings.ChatID)
	assert.True(t, settings.EnableAllJung)
	assert.False(t, settings.HasOffTime)
	assert.False(t, settings.HasWorkday)
}

func TestFromRowUsesStoredValues(t *testing.T) {
	enabled := false
	mask := workday.Mon | workday.Fri

	settings, err := FromRow(Row{
		ChatID:        123,
		ChatTitle:     "title",
		DateCreated:   "2019-03-16T02:26:19+08:00",
		TTL:           1558349640,
		EnableAllJung: &enabled,
		OffTime:       "1800",
		Workday:       &mask,
	})
	require.NoError(t, err)

	assert.Equal(t, int64(123), settings.ChatID)
	assert.Equal(t, "title", settings.ChatTitle)
	assert.Equal(t, "2019-03-16T02:26:19+08:00", settings.DateCreated.Format(time.RFC3339))
	assert.Equal(t, int64(1558349640), settings.TTL)
	assert.False(t, settings.EnableAllJung)
	assert.Equal(t, "1800", settings.OffTime)
	assert.True(t, settings.HasOffTime)
	assert.Equal(t, workday.Workdays(mask), settings.Workday)
	assert.True(t, settings.HasWorkday)
}

func TestFromRowRejectsMalformedValues(t *testing.T) {
	badMask := 128

	tests := []Row{
		{DateCreated: "bad-date"},
		{Workday: &badMask},
	}

	for _, test := range tests {
		_, err := FromRow(test)
		require.Error(t, err)
	}
}

func TestBuildMetadataUpdatePreservesContractShape(t *testing.T) {
	settings := Settings{
		ChatID:      123,
		ChatTitle:   "title",
		DateCreated: mustParseTime(t, "2019-04-01T02:38:24Z"),
		TTL:         1554691104,
	}

	update := BuildMetadataUpdate("chat-id-dev", settings)

	assert.Equal(t, "chat-id-dev", update.TableName)
	assert.Equal(t, map[string]any{"chatId": int64(123)}, update.Key)
	assert.Equal(t, "SET #ct = :ct, #dc = :dc, #ttl = :ttl", update.UpdateExpression)
	assert.Equal(t, map[string]string{"#ct": "chatTitle", "#dc": "dateCreated", "#ttl": "ttl"}, update.ExpressionAttributeNames)
	assert.Equal(t, map[string]any{":ct": "title", ":dc": "2019-04-01T10:38:24+08:00", ":ttl": int64(1554691104)}, update.ExpressionAttributeValues)
}

func TestBuildAllJungUpdatePreservesContractShape(t *testing.T) {
	update := BuildAllJungUpdate("chat-id-dev", 123, false)

	assert.Equal(t, "chat-id-dev", update.TableName)
	assert.Equal(t, map[string]any{"chatId": int64(123)}, update.Key)
	assert.Equal(t, "SET #eaj = :eaj", update.UpdateExpression)
	assert.Equal(t, map[string]string{"#eaj": "enableAllJung"}, update.ExpressionAttributeNames)
	assert.Equal(t, map[string]any{":eaj": false}, update.ExpressionAttributeValues)
}

func TestBuildOffWorkUpdatePreservesContractShape(t *testing.T) {
	update := BuildOffWorkUpdate("chat-id-dev", 123, "1800", workday.Workdays(workday.Mon|workday.Fri))

	assert.Equal(t, "chat-id-dev", update.TableName)
	assert.Equal(t, map[string]any{"chatId": int64(123)}, update.Key)
	assert.Equal(t, "SET #ot = :ot, #wd = :wd", update.UpdateExpression)
	assert.Equal(t, map[string]string{"#ot": "offTime", "#wd": "workday"}, update.ExpressionAttributeNames)
	assert.Equal(t, map[string]any{":ot": "1800", ":wd": workday.Mon | workday.Fri}, update.ExpressionAttributeValues)
}

func TestFilterDuePreservesContractDefaults(t *testing.T) {
	rows := []Settings{
		{ChatID: 1},
		{ChatID: 2, OffTime: "1000", HasOffTime: true, Workday: workday.Workdays(workday.Mon), HasWorkday: true},
		{ChatID: 3, OffTime: "1000", HasOffTime: true, Workday: workday.Workdays(workday.Tue), HasWorkday: true},
		{ChatID: 4, OffTime: "1800", HasOffTime: true, Workday: workday.Workdays(workday.Mon), HasWorkday: true},
		{ChatID: 5, OffTime: "1000", HasOffTime: true},
	}

	due := FilterDue(rows, "1000", "MON")

	assert.Equal(t, []Settings{rows[0], rows[1]}, due)
}

func TestFilterDueRejectsContractDefaultOutsideWeekday(t *testing.T) {
	due := FilterDue([]Settings{{ChatID: 1}}, "1000", "SAT")

	assert.Empty(t, due)
}

func TestFilterDueRejectsNonDefaultOffTimeForMissingSettings(t *testing.T) {
	due := FilterDue([]Settings{{ChatID: 1}}, "1800", "MON")

	assert.Empty(t, due)
}

func mustParseTime(t *testing.T, raw string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err)

	return parsed
}

type fakeChatClient struct {
	row       Row
	rows      []Row
	updates   []UpdateExpression
	missing   bool
	err       error
	getTable  string
	listTable string
	getChatID int64
}

func (client *fakeChatClient) Get(ctx context.Context, tableName string, chatID int64) (Row, bool, error) {
	client.getTable = tableName
	client.getChatID = chatID
	return client.row, !client.missing, client.err
}

func (client *fakeChatClient) Update(ctx context.Context, request UpdateExpression) error {
	client.updates = append(client.updates, request)
	return client.err
}

func (client *fakeChatClient) ListEnabled(ctx context.Context, tableName string) ([]Row, error) {
	client.listTable = tableName
	return client.rows, client.err
}
