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

	assert.Equal(t, "messages", queryRequest.TableName)
	assert.Equal(t, "chatId = :chat_id AND dateCreated > :date_created", queryRequest.KeyConditionExpression)
	assert.False(t, queryRequest.ScanIndexForward)
	assert.Equal(t, startKey, queryRequest.ExclusiveStartKey)
	assert.Equal(t, int64(123), queryRequest.ExpressionAttributeValues[":chat_id"])
	assert.Equal(t, "2026-04-25T20:30:00+08:00", queryRequest.ExpressionAttributeValues[":date_created"])
}
