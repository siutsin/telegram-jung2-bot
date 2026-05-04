package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	gomock "go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/message"
	mock "github.com/siutsin/telegram-jung2-bot/internal/mock"
)

func TestMessageClientSave(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	dynamoClient := mock.NewMockDynamoRequester(controller)
	dynamoClient.EXPECT().
		UpdateItem(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error) {
			assert.Equal(t, "messages", awscore.ToString(input.TableName))
			assert.NotNil(t, input.Key["chatId"])
			assert.NotNil(t, input.Key["dateCreated"])
			assert.NotNil(t, input.ExpressionAttributeValues[":ttl"])
			return &awsdynamodb.UpdateItemOutput{}, nil
		})

	err := NewMessageClient(dynamoClient).Save(context.Background(), "messages", message.Message{ChatID: 123})

	require.NoError(t, err)
}

func TestMessageClientQueryByChat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		queryErr    error
		item        map[string]ddbtypes.AttributeValue
		want        []message.Message
		wantErrText string
	}{
		{
			name: "success",
			item: map[string]ddbtypes.AttributeValue{
				"chatId":      &ddbtypes.AttributeValueMemberN{Value: "123"},
				"dateCreated": &ddbtypes.AttributeValueMemberS{Value: "2026-05-02T20:30:00+08:00"},
			},
			want: []message.Message{
				{ChatID: 123, DateCreated: time.Date(2026, 5, 2, 20, 30, 0, 0, time.FixedZone("", 8*60*60))},
			},
		},
		{name: "query error", queryErr: errors.New("boom"), wantErrText: "query DynamoDB messages: boom"},
		{
			name: "decode error",
			item: map[string]ddbtypes.AttributeValue{
				"dateCreated": &ddbtypes.AttributeValueMemberS{Value: "bad"},
			},
			wantErrText: "parse dateCreated",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			dynamoClient := mock.NewMockDynamoRequester(controller)
			dynamoClient.EXPECT().
				Query(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, input *awsdynamodb.QueryInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.QueryOutput, error) {
					assert.Equal(t, "messages", awscore.ToString(input.TableName))
					assert.Equal(t, "chatId = :chat_id AND dateCreated > :date_created", awscore.ToString(input.KeyConditionExpression))
					assert.False(t, awscore.ToBool(input.ScanIndexForward))
					if test.queryErr != nil {
						return nil, test.queryErr
					}
					return &awsdynamodb.QueryOutput{Items: []map[string]ddbtypes.AttributeValue{test.item}}, nil
				})

			rows, err := NewMessageClient(dynamoClient).QueryByChat(context.Background(), "messages", 123, time.Date(2026, 5, 2, 12, 30, 0, 0, time.UTC))

			if test.wantErrText != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.wantErrText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, rows)
		})
	}
}

func TestQueryMessagesRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 2, 12, 30, 0, 0, time.UTC)
	startKey := map[string]any{"chatId": int64(123), "dateCreated": "cursor"}

	queryRequest := queryMessagesRequest("messages", 123, now.AddDate(0, 0, -7), startKey)

	assert.Equal(t, "messages", queryRequest.tableName)
	assert.Equal(t, "chatId = :chat_id AND dateCreated > :date_created", queryRequest.keyConditionExpression)
	assert.False(t, queryRequest.scanIndexForward)
	assert.Equal(t, startKey, queryRequest.exclusiveStartKey)
	assert.Equal(t, int64(123), queryRequest.expressionAttributeValues[":chat_id"])
	assert.Equal(t, "2026-04-25T20:30:00+08:00", queryRequest.expressionAttributeValues[":date_created"])
}

func TestMessageClientSaveMatchesLegacyBuildExpression(t *testing.T) {
	t.Parallel()

	// Mirrors legacy/src/dynamodb.js buildExpression and legacy/test/testDynamodb.js.
	tests := []struct {
		name string
		row  message.Message
		want itemUpdateRequest
	}{
		{
			name: "all attributes",
			row: message.Message{
				ChatID:      123,
				DateCreated: mustParseDynamoDBTime(t),
				ChatTitle:   "title",
				UserID:      234,
				Username:    "username",
				FirstName:   "first_name",
				LastName:    "last_name",
				TTL:         1554691104,
			},
			want: itemUpdateRequest{
				tableName: "messages-dev",
				key: map[string]any{
					"chatId":      int64(123),
					"dateCreated": "2019-04-01T10:38:24+08:00",
				},
				updateExpression: "SET #chatTitle = :chatTitle, #userId = :userId, #username = :username, #firstName = :firstName, #lastName = :lastName, #ttl = :ttl",
				expressionAttributeNames: map[string]string{
					"#chatTitle": "chatTitle",
					"#firstName": "firstName",
					"#lastName":  "lastName",
					"#ttl":       "ttl",
					"#userId":    "userId",
					"#username":  "username",
				},
				expressionAttributeValues: map[string]any{
					":chatTitle": "title",
					":firstName": "first_name",
					":lastName":  "last_name",
					":ttl":       int64(1554691104),
					":userId":    int64(234),
					":username":  "username",
				},
			},
		},
		{
			name: "omits missing optional attributes",
			row: message.Message{
				ChatID:      123,
				DateCreated: mustParseDynamoDBTime(t),
				TTL:         1554691104,
			},
			want: itemUpdateRequest{
				tableName: "messages-dev",
				key: map[string]any{
					"chatId":      int64(123),
					"dateCreated": "2019-04-01T10:38:24+08:00",
				},
				updateExpression:          "SET #ttl = :ttl",
				expressionAttributeNames:  map[string]string{"#ttl": "ttl"},
				expressionAttributeValues: map[string]any{":ttl": int64(1554691104)},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			dynamoClient := mock.NewMockDynamoRequester(controller)
			dynamoClient.EXPECT().
				UpdateItem(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, input *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error) {
					assert.Equal(t, test.want.tableName, awscore.ToString(input.TableName))
					assert.Equal(t, test.want.key, decodeLastEvaluatedKey(input.Key))
					assert.Equal(t, test.want.updateExpression, awscore.ToString(input.UpdateExpression))
					assert.Equal(t, test.want.expressionAttributeNames, input.ExpressionAttributeNames)
					assert.Equal(t, test.want.expressionAttributeValues, decodeLastEvaluatedKey(input.ExpressionAttributeValues))
					return &awsdynamodb.UpdateItemOutput{}, nil
				})

			err := NewMessageClient(dynamoClient).Save(context.Background(), "messages-dev", test.row)

			require.NoError(t, err)
		})
	}
}

func TestIsZeroMessageAttributeValue(t *testing.T) {
	t.Parallel()

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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, isZeroMessageAttributeValue(test.value))
		})
	}
}

func mustParseDynamoDBTime(t *testing.T) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, "2019-04-01T02:38:24Z")
	require.NoError(t, err)

	return parsed
}
