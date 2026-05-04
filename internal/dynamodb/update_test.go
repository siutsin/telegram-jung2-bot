package dynamodb

import (
	"context"
	"errors"
	"testing"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	gomock "go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mock "github.com/siutsin/telegram-jung2-bot/internal/mock"
)

func TestUpdateItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		updateErr   error
		wantErrText string
	}{
		{name: "success"},
		{name: "sdk error", updateErr: errors.New("boom"), wantErrText: "update DynamoDB item: boom"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			dynamoClient := mock.NewMockDynamoRequester(controller)
			wantInput := &awsdynamodb.UpdateItemInput{
				TableName: awscore.String("chats"),
				Key: map[string]ddbtypes.AttributeValue{
					"chatId": &ddbtypes.AttributeValueMemberN{Value: "42"},
				},
				UpdateExpression: awscore.String("SET #ot = :ot"),
				ExpressionAttributeNames: map[string]string{
					"#ot": "offTime",
				},
				ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
					":ot": &ddbtypes.AttributeValueMemberS{Value: "1830"},
				},
			}

			dynamoClient.EXPECT().
				UpdateItem(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, input *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error) {
					assert.Equal(t, wantInput, input)
					if test.updateErr != nil {
						return nil, test.updateErr
					}
					return &awsdynamodb.UpdateItemOutput{}, nil
				})

			err := updateItem(context.Background(), dynamoClient, itemUpdateRequest{
				tableName:        "chats",
				key:              map[string]any{"chatId": int64(42)},
				updateExpression: "SET #ot = :ot",
				expressionAttributeNames: map[string]string{
					"#ot": "offTime",
				},
				expressionAttributeValues: map[string]any{
					":ot": "1830",
				},
			})

			if test.wantErrText != "" {
				require.Error(t, err)
				assert.EqualError(t, err, test.wantErrText)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestUpdateContractUpdate(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	dynamoClient := mock.NewMockDynamoRequester(controller)
	wantInput := &awsdynamodb.UpdateItemInput{
		TableName: awscore.String("chats"),
		Key: map[string]ddbtypes.AttributeValue{
			"chatId": &ddbtypes.AttributeValueMemberN{Value: "42"},
		},
		UpdateExpression: awscore.String("SET #ot = :ot"),
		ExpressionAttributeNames: map[string]string{
			"#ot": "offTime",
		},
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":ot": &ddbtypes.AttributeValueMemberS{Value: "1830"},
		},
	}
	dynamoClient.EXPECT().
		UpdateItem(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error) {
			assert.Equal(t, wantInput, input)
			return &awsdynamodb.UpdateItemOutput{}, nil
		})

	err := updateContractUpdate(
		context.Background(),
		dynamoClient,
		"chats",
		map[string]any{"chatId": int64(42)},
		"SET #ot = :ot",
		map[string]string{"#ot": "offTime"},
		map[string]any{":ot": "1830"},
	)

	require.NoError(t, err)
}
