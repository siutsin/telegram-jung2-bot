package dynamodb

import (
	"context"
	"fmt"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/siutsin/telegram-jung2-bot/internal/message"
)

type messageQueryRequest struct {
	TableName                 string
	KeyConditionExpression    string
	ScanIndexForward          bool
	ExclusiveStartKey         map[string]any
	ExpressionAttributeValues map[string]any
}

// Save stores a message row in DynamoDB.
func (client MessageClient) Save(ctx context.Context, tableName string, row message.Message) error {
	if row.DateCreated.IsZero() {
		row.DateCreated = time.Now()
	}
	if row.TTL == 0 {
		row.TTL = message.TTL(row.DateCreated, message.DefaultTTL)
	}

	saveUpdate := message.BuildSaveUpdate(tableName, row)
	return updateContractUpdate(ctx, client.dynamo, saveUpdate.TableName, saveUpdate.Key, saveUpdate.UpdateExpression, saveUpdate.ExpressionAttributeNames, saveUpdate.ExpressionAttributeValues)
}

// QueryByChat loads message rows for one chat.
func (client MessageClient) QueryByChat(ctx context.Context, tableName string, chatID int64, since time.Time) ([]message.Message, error) {
	return collectPages(ctx, func(ctx context.Context, startKey map[string]any) (page[message.Message], error) {
		queryRequest := queryMessagesRequest(tableName, chatID, since, startKey)
		output, err := client.dynamo.Query(ctx, &awsdynamodb.QueryInput{
			TableName:                 awscore.String(queryRequest.TableName),
			ExclusiveStartKey:         encodeDynamoValues(queryRequest.ExclusiveStartKey),
			KeyConditionExpression:    awscore.String(queryRequest.KeyConditionExpression),
			ScanIndexForward:          awscore.Bool(queryRequest.ScanIndexForward),
			ExpressionAttributeValues: encodeDynamoValues(queryRequest.ExpressionAttributeValues),
		})
		if err != nil {
			return page[message.Message]{}, fmt.Errorf("query DynamoDB messages: %w", err)
		}

		rows := make([]message.Message, 0, len(output.Items))
		for _, item := range output.Items {
			row, err := decodeMessage(item)
			if err != nil {
				return page[message.Message]{}, err
			}
			rows = append(rows, row)
		}
		return page[message.Message]{
			Items:            rows,
			LastEvaluatedKey: decodeLastEvaluatedKey(output.LastEvaluatedKey),
		}, nil
	})
}

// queryMessagesRequest builds a message query request.
// For example, chatID 42 becomes a query keyed by chatId 42 and dateCreated >
// the requested cutoff.
func queryMessagesRequest(tableName string, chatID int64, since time.Time, startKey map[string]any) messageQueryRequest {
	return messageQueryRequest{
		TableName:              tableName,
		KeyConditionExpression: "chatId = :chat_id AND dateCreated > :date_created",
		ScanIndexForward:       false,
		ExclusiveStartKey:      startKey,
		ExpressionAttributeValues: map[string]any{
			":chat_id":      chatID,
			":date_created": message.FormatDateCreated(since),
		},
	}
}
