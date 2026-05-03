package dynamodb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

// API is the DynamoDB SDK surface used by the store adapters.
type API interface {
	DescribeTable(ctx context.Context, params *awsdynamodb.DescribeTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.DescribeTableOutput, error)
	GetItem(ctx context.Context, params *awsdynamodb.GetItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.GetItemOutput, error)
	Query(ctx context.Context, params *awsdynamodb.QueryInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.QueryOutput, error)
	Scan(ctx context.Context, params *awsdynamodb.ScanInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.ScanOutput, error)
	UpdateItem(ctx context.Context, params *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error)
	UpdateTable(ctx context.Context, params *awsdynamodb.UpdateTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateTableOutput, error)
}

// MessageClient adapts DynamoDB to the message repository contract.
type MessageClient struct {
	Dynamo API
}

// ChatClient adapts DynamoDB to the chat repository contract.
type ChatClient struct {
	Dynamo API
}

// ScaleUpper raises DynamoDB capacity for the message table.
type ScaleUpper struct {
	Dynamo      API
	DesiredRead int
	TableName   string
}

const allWorkdays = workday.Sun | workday.Mon | workday.Tue | workday.Wed | workday.Thu | workday.Fri | workday.Sat

// Update stores a message row in DynamoDB.
func (client MessageClient) Update(ctx context.Context, request message.UpdateExpression) error {
	return updateItem(ctx, client.Dynamo, Request{
		ExpressionAttributeNames:  request.ExpressionAttributeNames,
		ExpressionAttributeValues: request.ExpressionAttributeValues,
		Key:                       request.Key,
		TableName:                 request.TableName,
		UpdateExpression:          request.UpdateExpression,
	})
}

// QueryByChat loads message rows for one chat.
func (client MessageClient) QueryByChat(ctx context.Context, request message.QueryRequest) ([]message.Message, error) {
	rows := make([]message.Message, 0)
	startKey := map[string]ddbtypes.AttributeValue(nil)

	for {
		output, err := client.Dynamo.Query(ctx, &awsdynamodb.QueryInput{
			TableName:              awscore.String(request.TableName),
			ExclusiveStartKey:      startKey,
			KeyConditionExpression: awscore.String("chatId = :chat_id AND dateCreated > :date_created"),
			ScanIndexForward:       awscore.Bool(!request.Descending),
			ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
				":chat_id":      &ddbtypes.AttributeValueMemberN{Value: strconv.FormatInt(request.ChatID, 10)},
				":date_created": &ddbtypes.AttributeValueMemberS{Value: message.FormatDateCreated(request.Since)},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("query DynamoDB messages: %w", err)
		}

		for _, item := range output.Items {
			row, err := decodeMessage(item)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}

		if len(output.LastEvaluatedKey) == 0 {
			return rows, nil
		}
		startKey = output.LastEvaluatedKey
	}
}

// Get loads one chat settings row.
func (client ChatClient) Get(ctx context.Context, tableName string, chatID int64) (chat.Row, bool, error) {
	output, err := client.Dynamo.GetItem(ctx, &awsdynamodb.GetItemInput{
		TableName: awscore.String(tableName),
		Key: map[string]ddbtypes.AttributeValue{
			"chatId": &ddbtypes.AttributeValueMemberN{Value: strconv.FormatInt(chatID, 10)},
		},
	})
	if err != nil {
		return chat.Row{}, false, fmt.Errorf("get DynamoDB chat row: %w", err)
	}
	if len(output.Item) == 0 {
		return chat.Row{}, false, nil
	}

	return decodeChat(output.Item), true, nil
}

// Update stores a chat settings row.
func (client ChatClient) Update(ctx context.Context, request chat.UpdateExpression) error {
	return updateItem(ctx, client.Dynamo, Request{
		ExpressionAttributeNames:  request.ExpressionAttributeNames,
		ExpressionAttributeValues: request.ExpressionAttributeValues,
		Key:                       request.Key,
		TableName:                 request.TableName,
		UpdateExpression:          request.UpdateExpression,
	})
}

// ListEnabled scans chat settings rows for scheduling.
func (client ChatClient) ListEnabled(ctx context.Context, tableName string) ([]chat.Row, error) {
	rows := make([]chat.Row, 0)
	startKey := map[string]ddbtypes.AttributeValue(nil)

	for {
		output, err := client.Dynamo.Scan(ctx, &awsdynamodb.ScanInput{
			TableName:         awscore.String(tableName),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, fmt.Errorf("scan DynamoDB chat rows: %w", err)
		}

		for _, item := range output.Items {
			rows = append(rows, decodeChat(item))
		}

		if len(output.LastEvaluatedKey) == 0 {
			return rows, nil
		}
		startKey = output.LastEvaluatedKey
	}
}

// SaveStatistics stores the computed group counts in the chat row.
func (client ChatClient) SaveStatistics(ctx context.Context, tableName string, chatID int64, userCount int, messageCount int, now time.Time) error {
	return updateItem(ctx, client.Dynamo, BuildChatCountUpdate(tableName, chatID, userCount, messageCount, now))
}

// DueChatIDs scans and filters due chats for one scheduled window.
func (client ChatClient) DueChatIDs(ctx context.Context, tableName string, timestamp time.Time) ([]int64, error) {
	window := schedule.WindowFromTime(timestamp)
	request := ScanDueChatsRequest(tableName, window.OffTime, window.Weekday, nil)
	startKey := map[string]ddbtypes.AttributeValue(nil)
	chatIDs := make([]int64, 0)

	for {
		output, err := client.Dynamo.Scan(ctx, &awsdynamodb.ScanInput{
			TableName:                 awscore.String(request.TableName),
			ExclusiveStartKey:         startKey,
			FilterExpression:          awscore.String(request.FilterExpression),
			ExpressionAttributeNames:  request.ExpressionAttributeNames,
			ExpressionAttributeValues: encodeDynamoValues(request.ExpressionAttributeValues),
		})
		if err != nil {
			return nil, fmt.Errorf("scan due chats: %w", err)
		}

		for _, item := range output.Items {
			row := decodeChat(item)
			if dueScanRowMatches(row, window.Weekday) {
				chatIDs = append(chatIDs, row.ChatID)
			}
		}

		if len(output.LastEvaluatedKey) == 0 {
			return chatIDs, nil
		}
		startKey = output.LastEvaluatedKey
	}
}

// ScaleUp raises DynamoDB read capacity to the configured target.
func (service ScaleUpper) ScaleUp(ctx context.Context) error {
	output, err := service.Dynamo.DescribeTable(ctx, &awsdynamodb.DescribeTableInput{
		TableName: awscore.String(service.TableName),
	})
	if err != nil {
		return fmt.Errorf("describe DynamoDB table: %w", err)
	}

	throughput := output.Table.ProvisionedThroughput
	request := BuildScaleUpRequest(
		service.TableName,
		awscore.ToInt64(throughput.ReadCapacityUnits),
		awscore.ToInt64(throughput.WriteCapacityUnits),
		strconv.Itoa(service.DesiredRead),
	)

	_, err = service.Dynamo.UpdateTable(ctx, &awsdynamodb.UpdateTableInput{
		TableName: awscore.String(request.TableName),
		ProvisionedThroughput: &ddbtypes.ProvisionedThroughput{
			ReadCapacityUnits:  awscore.Int64(request.ProvisionedThroughput.ReadCapacityUnits),
			WriteCapacityUnits: awscore.Int64(request.ProvisionedThroughput.WriteCapacityUnits),
		},
	})
	if err != nil {
		if IsIgnoredScaleUpError(err) {
			return nil
		}
		return fmt.Errorf("update DynamoDB table: %w", err)
	}

	return nil
}

// updateItem applies a contract update expression in DynamoDB.
func updateItem(ctx context.Context, dynamoClient API, request Request) error {
	_, err := dynamoClient.UpdateItem(ctx, &awsdynamodb.UpdateItemInput{
		TableName:                 awscore.String(request.TableName),
		Key:                       encodeDynamoValues(request.Key),
		UpdateExpression:          awscore.String(request.UpdateExpression),
		ExpressionAttributeNames:  request.ExpressionAttributeNames,
		ExpressionAttributeValues: encodeDynamoValues(request.ExpressionAttributeValues),
	})
	if err != nil {
		return fmt.Errorf("update DynamoDB item: %w", err)
	}

	return nil
}

// encodeDynamoValues converts loose contract values into DynamoDB attributes.
func encodeDynamoValues(values map[string]any) map[string]ddbtypes.AttributeValue {
	encoded := make(map[string]ddbtypes.AttributeValue, len(values))
	for name, value := range values {
		encoded[name] = encodeDynamoValue(value)
	}

	return encoded
}

// encodeDynamoValue converts one loose contract value into a DynamoDB attribute.
func encodeDynamoValue(value any) ddbtypes.AttributeValue {
	switch typed := value.(type) {
	case bool:
		return &ddbtypes.AttributeValueMemberBOOL{Value: typed}
	case float64:
		return &ddbtypes.AttributeValueMemberN{Value: strconv.FormatFloat(typed, 'f', -1, 64)}
	case int:
		return &ddbtypes.AttributeValueMemberN{Value: strconv.Itoa(typed)}
	case int64:
		return &ddbtypes.AttributeValueMemberN{Value: strconv.FormatInt(typed, 10)}
	case string:
		return &ddbtypes.AttributeValueMemberS{Value: typed}
	default:
		return &ddbtypes.AttributeValueMemberS{Value: fmt.Sprint(value)}
	}
}

// decodeMessage converts one DynamoDB item into a stored message row.
func decodeMessage(item map[string]ddbtypes.AttributeValue) (message.Message, error) {
	timestamp, err := message.ParseDateCreated(stringAttribute(item, "dateCreated"))
	if err != nil {
		return message.Message{}, err
	}

	return message.Message{
		ChatID:      int64Attribute(item, "chatId"),
		ChatTitle:   stringAttribute(item, "chatTitle"),
		DateCreated: timestamp,
		FirstName:   stringAttribute(item, "firstName"),
		LastName:    stringAttribute(item, "lastName"),
		TTL:         int64Attribute(item, "ttl"),
		UserID:      int64Attribute(item, "userId"),
		Username:    stringAttribute(item, "username"),
	}, nil
}

// decodeChat converts one DynamoDB item into a chat row.
func decodeChat(item map[string]ddbtypes.AttributeValue) chat.Row {
	return chat.Row{
		ChatID:        int64Attribute(item, "chatId"),
		ChatTitle:     stringAttribute(item, "chatTitle"),
		DateCreated:   stringAttribute(item, "dateCreated"),
		TTL:           int64Attribute(item, "ttl"),
		EnableAllJung: boolAttribute(item, "enableAllJung"),
		OffTime:       stringAttribute(item, "offTime"),
		Workday:       intAttribute(item, "workday"),
	}
}

// dueScanRowMatches applies the reference post-scan weekday filter.
func dueScanRowMatches(row chat.Row, day string) bool {
	if row.Workday == nil {
		return row.OffTime == ""
	}

	return workday.MatchesDay(day, workday.Workdays(*row.Workday&allWorkdays))
}

// stringAttribute returns a string attribute when present.
func stringAttribute(item map[string]ddbtypes.AttributeValue, key string) string {
	attribute, ok := item[key]
	if !ok {
		return ""
	}

	value, ok := attribute.(*ddbtypes.AttributeValueMemberS)
	if !ok {
		return ""
	}

	return value.Value
}

// int64Attribute returns an int64 attribute when present.
func int64Attribute(item map[string]ddbtypes.AttributeValue, key string) int64 {
	attribute, ok := item[key]
	if !ok {
		return 0
	}

	value, ok := attribute.(*ddbtypes.AttributeValueMemberN)
	if !ok {
		return 0
	}

	parsed, err := strconv.ParseInt(value.Value, 10, 64)
	if err != nil {
		return 0
	}

	return parsed
}

// intAttribute returns an int attribute when present.
func intAttribute(item map[string]ddbtypes.AttributeValue, key string) *int {
	attribute, ok := item[key]
	if !ok {
		return nil
	}

	value, ok := attribute.(*ddbtypes.AttributeValueMemberN)
	if !ok {
		return nil
	}

	parsed, err := strconv.Atoi(value.Value)
	if err != nil {
		return nil
	}

	return &parsed
}

// boolAttribute returns a bool attribute when present.
func boolAttribute(item map[string]ddbtypes.AttributeValue, key string) *bool {
	attribute, ok := item[key]
	if !ok {
		return nil
	}

	value, ok := attribute.(*ddbtypes.AttributeValueMemberBOOL)
	if !ok {
		return nil
	}

	parsed := value.Value
	return &parsed
}
