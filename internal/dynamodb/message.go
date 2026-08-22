package dynamodb

import (
	"context"
	"fmt"
	"strings"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/siutsin/telegram-jung2-bot/internal/message"
)

type messageQueryRequest struct {
	tableName                 string
	keyConditionExpression    string
	scanIndexForward          bool
	exclusiveStartKey         map[string]any
	expressionAttributeValues map[string]any
}

// Save stores a message row in DynamoDB.
func (client MessageClient) Save(ctx context.Context, tableName string, row message.Message) error {
	if row.DateCreated.IsZero() {
		row.DateCreated = time.Now()
	}
	if row.TTL == 0 {
		row.TTL = message.TTL(row.DateCreated, message.DefaultTTL)
	}

	return updateItem(ctx, client.dynamo, buildMessageSaveUpdate(tableName, row))
}

// QueryByChat loads message rows for one chat.
func (client MessageClient) QueryByChat(ctx context.Context, tableName string, chatID int64, since time.Time) ([]message.Message, error) {
	return collectPages(ctx, func(pageCtx context.Context, startKey map[string]any) (page[message.Message], error) {
		queryRequest := queryMessagesRequest(tableName, chatID, since, startKey)
		output, err := client.dynamo.Query(pageCtx, &awsdynamodb.QueryInput{
			TableName:                 awscore.String(queryRequest.tableName),
			ExclusiveStartKey:         encodeDynamoValues(queryRequest.exclusiveStartKey),
			KeyConditionExpression:    awscore.String(queryRequest.keyConditionExpression),
			ScanIndexForward:          awscore.Bool(queryRequest.scanIndexForward),
			ExpressionAttributeValues: encodeDynamoValues(queryRequest.expressionAttributeValues),
		})
		if err != nil {
			return page[message.Message]{}, fmt.Errorf("query DynamoDB messages: %w", err)
		}

		rows := make([]message.Message, 0, len(output.Items))
		for _, item := range output.Items {
			row, decodeErr := decodeMessage(item)
			if decodeErr != nil {
				return page[message.Message]{}, decodeErr
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
		tableName:              tableName,
		keyConditionExpression: "chatId = :chat_id AND dateCreated > :date_created",
		scanIndexForward:       false,
		exclusiveStartKey:      startKey,
		expressionAttributeValues: map[string]any{
			":chat_id":      chatID,
			":date_created": message.FormatDateCreated(since),
		},
	}
}

// buildMessageSaveUpdate builds the contract message update request.
// For example, a message with username and firstName adds only those non-empty
// fields to the SET clause.
func buildMessageSaveUpdate(tableName string, row message.Message) itemUpdateRequest {
	attributeNames := make(map[string]string)
	attributeValues := make(map[string]any)
	assignments := make([]string, 0, 6)

	attributes := []struct {
		name  string
		value any
	}{
		{name: "chatTitle", value: row.ChatTitle},
		{name: "userId", value: row.UserID},
		{name: "username", value: row.Username},
		{name: "firstName", value: row.FirstName},
		{name: "lastName", value: row.LastName},
		{name: "ttl", value: row.TTL},
	}
	for _, attribute := range attributes {
		assignments = addMessageAttribute(assignments, attributeNames, attributeValues, attribute.name, attribute.value)
	}

	return itemUpdateRequest{
		tableName: tableName,
		key: map[string]any{
			"chatId":      row.ChatID,
			"dateCreated": message.FormatDateCreated(row.DateCreated),
		},
		updateExpression:          "SET " + strings.Join(assignments, ", "),
		expressionAttributeNames:  attributeNames,
		expressionAttributeValues: attributeValues,
	}
}

// addMessageAttribute adds a non-zero contract attribute to an update.
// For example, "username", "alice" appends "#username = :username".
func addMessageAttribute(
	assignments []string,
	names map[string]string,
	values map[string]any,
	name string,
	value any,
) []string {
	if isZeroMessageAttributeValue(value) {
		return assignments
	}

	placeholder := "#" + name
	valuePlaceholder := ":" + name
	names[placeholder] = name
	values[valuePlaceholder] = value
	return append(assignments, placeholder+" = "+valuePlaceholder)
}

func isZeroMessageAttributeValue(value any) bool {
	switch typedValue := value.(type) {
	case string:
		return typedValue == ""
	case int64:
		return typedValue == 0
	default:
		return value == nil
	}
}
