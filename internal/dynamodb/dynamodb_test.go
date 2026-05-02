package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectPages(t *testing.T) {
	t.Parallel()

	pageIndex := 0
	rows, err := CollectPages(context.Background(), func(ctx context.Context, startKey map[string]any) (Page[string], error) {
		assert.NoError(t, ctx.Err())
		pageIndex++
		switch pageIndex {
		case 1:
			assert.Nil(t, startKey)
			return Page[string]{
				Items:            []string{"first"},
				LastEvaluatedKey: map[string]any{"chatId": int64(1)},
			}, nil
		case 2:
			assert.Equal(t, map[string]any{"chatId": int64(1)}, startKey)
			return Page[string]{Items: []string{"second"}}, nil
		default:
			t.Fatalf("unexpected page %d", pageIndex)
			return Page[string]{}, nil
		}
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, rows)
}

func TestCollectPagesEmpty(t *testing.T) {
	t.Parallel()

	rows, err := CollectPages(context.Background(), func(ctx context.Context, startKey map[string]any) (Page[string], error) {
		assert.Nil(t, startKey)
		return Page[string]{}, nil
	})

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestCollectPagesPropagatesFetchError(t *testing.T) {
	t.Parallel()

	_, err := CollectPages(context.Background(), func(ctx context.Context, startKey map[string]any) (Page[string], error) {
		return Page[string]{}, errors.New("boom")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "collect dynamodb pages")
	assert.Contains(t, err.Error(), "boom")
}

func TestCollectPagesPropagatesContextError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CollectPages(ctx, func(ctx context.Context, startKey map[string]any) (Page[string], error) {
		t.Fatal("fetch should not run after context cancellation")
		return Page[string]{}, nil
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestQueryMessagesRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 2, 12, 30, 0, 0, time.UTC)
	startKey := map[string]any{"chatId": int64(123), "dateCreated": "cursor"}

	request := QueryMessagesRequest("messages", 123, now, 7, startKey)

	assert.Equal(t, "messages", request.TableName)
	assert.Equal(t, "chatId = :chat_id AND dateCreated > :date_created", request.KeyConditionExpression)
	assert.False(t, request.ScanIndexForward)
	assert.Equal(t, startKey, request.ExclusiveStartKey)
	assert.Equal(t, int64(123), request.ExpressionAttributeValues[":chat_id"])
	assert.Equal(t, "2026-04-25T20:30:00+08:00", request.ExpressionAttributeValues[":date_created"])
}

func TestQueryChatStatsRequest(t *testing.T) {
	t.Parallel()

	request := QueryChatStatsRequest("chats", 456)

	assert.Equal(t, "chats", request.TableName)
	assert.Equal(t, "chatId = :chat_id", request.KeyConditionExpression)
	assert.Equal(t, int64(456), request.ExpressionAttributeValues[":chat_id"])
}

func TestScanDueChatsRequest(t *testing.T) {
	t.Parallel()

	request := ScanDueChatsRequest("chats", "1800", "MON", map[string]any{"chatId": int64(1)})

	assert.Equal(t, "chats", request.TableName)
	assert.Equal(t, "#ot = :ot", request.FilterExpression)
	assert.Equal(t, map[string]string{"#ot": "offTime"}, request.ExpressionAttributeNames)
	assert.Equal(t, map[string]any{":ot": "1800"}, request.ExpressionAttributeValues)
	assert.Equal(t, map[string]any{"chatId": int64(1)}, request.ExclusiveStartKey)
}

func TestScanDueChatsRequestIncludesContractDefaults(t *testing.T) {
	t.Parallel()

	request := ScanDueChatsRequest("chats", "1000", "FRI", nil)

	assert.Equal(t, "#ot = :ot Or (attribute_not_exists(#ot) And attribute_not_exists(#wd))", request.FilterExpression)
	assert.Equal(t, map[string]string{
		"#ot": "offTime",
		"#wd": "workday",
	}, request.ExpressionAttributeNames)
}

func TestScanDueChatsRequestSkipsContractDefaultsOnWeekend(t *testing.T) {
	t.Parallel()

	request := ScanDueChatsRequest("chats", "1000", "SAT", nil)

	assert.Equal(t, "#ot = :ot", request.FilterExpression)
	assert.Equal(t, map[string]string{"#ot": "offTime"}, request.ExpressionAttributeNames)
}

func TestBuildChatCountUpdate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 2, 12, 30, 0, 0, time.UTC)
	request := BuildChatCountUpdate("chats", 123, 2, 5, now)

	assert.Equal(t, "chats", request.TableName)
	assert.Equal(t, map[string]any{"chatId": int64(123)}, request.Key)
	assert.Equal(t, "SET #uc = :uc, #mc = :mc, #mpu = :mpu, #ct = :ct", request.UpdateExpression)
	assert.Equal(t, map[string]string{
		"#uc":  "userCount",
		"#mc":  "messageCount",
		"#mpu": "messagePerUser",
		"#ct":  "countTimestamp",
	}, request.ExpressionAttributeNames)
	assert.Equal(t, map[string]any{
		":uc":  2,
		":mc":  5,
		":mpu": 2.5,
		":ct":  "2026-05-02T20:30:00+08:00",
	}, request.ExpressionAttributeValues)
}

func TestBuildScaleUpRequest(t *testing.T) {
	t.Parallel()

	request := BuildScaleUpRequest("messages", 1, 3, "10")

	assert.Equal(t, "messages", request.TableName)
	assert.Equal(t, Throughput{ReadCapacityUnits: 10, WriteCapacityUnits: 3}, request.ProvisionedThroughput)
}

func TestBuildScaleUpRequestUsesCurrentReadForInvalidDesiredRead(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(1), BuildScaleUpRequest("messages", 1, 3, "some string").ProvisionedThroughput.ReadCapacityUnits)
	assert.Equal(t, int64(1), BuildScaleUpRequest("messages", 1, 3, "").ProvisionedThroughput.ReadCapacityUnits)
	assert.Equal(t, int64(1), BuildScaleUpRequest("messages", 1, 3, "0").ProvisionedThroughput.ReadCapacityUnits)
}

func TestIsIgnoredScaleUpError(t *testing.T) {
	t.Parallel()

	assert.False(t, IsIgnoredScaleUpError(nil))
	assert.True(t, IsIgnoredScaleUpError(errors.New("Subscriber limit exceeded: daily limit")))
	assert.True(t, IsIgnoredScaleUpError(errors.New("The provisioned throughput for the table will not change blah")))
	assert.True(t, IsIgnoredScaleUpError(errors.New("Attempt to change a resource which is still in use blah")))
	assert.False(t, IsIgnoredScaleUpError(errors.New("Some other errors")))
}

func TestSanitisedLogFields(t *testing.T) {
	t.Parallel()

	request := Request{
		TableName:              "messages",
		Key:                    map[string]any{"chatId": int64(123)},
		KeyConditionExpression: "chatId = :chat_id",
		FilterExpression:       "#ot = :ot",
		ExclusiveStartKey:      map[string]any{"cursor": "next"},
		ExpressionAttributeValues: map[string]any{
			":messageText": "private text",
		},
	}

	assert.Equal(t, map[string]any{
		"tableName":              "messages",
		"key":                    map[string]any{"chatId": int64(123)},
		"keyConditionExpression": "chatId = :chat_id",
		"filterExpression":       "#ot = :ot",
		"hasExclusiveStartKey":   true,
	}, SanitisedLogFields(request))
}

func TestSanitisedLogFieldsMinimal(t *testing.T) {
	t.Parallel()

	assert.Equal(t, map[string]any{"tableName": "messages"}, SanitisedLogFields(Request{TableName: "messages"}))
}
