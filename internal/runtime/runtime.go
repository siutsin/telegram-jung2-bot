// Package runtime wires production adapters and queue handlers.
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
	contractdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/statistics"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

// Components contains the assembled production runtime dependencies.
type Components struct {
	Store      httpserver.Store
	Sender     queue.Sender
	Receiver   queue.Receiver
	Deleter    worker.Deleter
	Messenger  httpserver.Messenger
	ScaleUpper httpserver.ScaleUpper
	Handlers   worker.Handlers
	Now        func() time.Time
}

type dynamoAPI interface {
	DescribeTable(ctx context.Context, params *awsdynamodb.DescribeTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.DescribeTableOutput, error)
	GetItem(ctx context.Context, params *awsdynamodb.GetItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.GetItemOutput, error)
	Query(ctx context.Context, params *awsdynamodb.QueryInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.QueryOutput, error)
	Scan(ctx context.Context, params *awsdynamodb.ScanInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.ScanOutput, error)
	UpdateItem(ctx context.Context, params *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error)
	UpdateTable(ctx context.Context, params *awsdynamodb.UpdateTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateTableOutput, error)
}

type sqsAPI interface {
	DeleteMessage(ctx context.Context, params *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error)
	ReceiveMessage(ctx context.Context, params *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error)
	SendMessage(ctx context.Context, params *awssqs.SendMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error)
}

type telegramAPI interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageWithOptions(ctx context.Context, chatID int64, text string, options telegram.SendMessageOptions) error
	IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error)
}

type store struct {
	messages message.Repository
	chats    chat.Repository
}

type messageClient struct {
	dynamo dynamoAPI
}

type chatClient struct {
	dynamo dynamoAPI
}

type sqsClient struct {
	queue sqsAPI
}

type scaleUpService struct {
	dynamo      dynamoAPI
	desiredRead int
	tableName   string
}

type actionService struct {
	chatClient         chat.RepositoryClient
	chatRepository     chat.Repository
	chatTable          string
	dynamo             dynamoAPI
	messageRepository  message.Repository
	now                func() time.Time
	telegramClient     telegramAPI
	updateCountsClient dynamoAPI
	sender             queue.Sender
	queueURL           string
}

type updateRequest struct {
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]any
	Key                       map[string]any
	TableName                 string
	UpdateExpression          string
}

const allWorkdays = workday.Sun | workday.Mon | workday.Tue | workday.Wed | workday.Thu | workday.Fri | workday.Sat

// NewComponents builds the production runtime dependencies.
func NewComponents(ctx context.Context, serviceConfig config.Config) (Components, error) {
	awsConfig, err := loadAWSConfig(ctx, serviceConfig)
	if err != nil {
		return Components{}, fmt.Errorf("load AWS config: %w", err)
	}

	dynamoClient := awsdynamodb.NewFromConfig(awsConfig, dynamoOptions(serviceConfig)...)
	queueClient := awssqs.NewFromConfig(awsConfig, sqsOptions(serviceConfig)...)
	telegramClient := telegram.NewClient(
		serviceConfig.TelegramBotToken,
		telegram.WithBaseURL(serviceConfig.TelegramAPIBaseURL),
		telegram.WithHTTPClient(&http.Client{Timeout: serviceConfig.HTTPTimeout}),
	)

	return buildComponents(serviceConfig, dynamoClient, queueClient, telegramClient), nil
}

// SaveMessage persists a Telegram message row.
func (store store) SaveMessage(ctx context.Context, request message.UpdateExpression) error {
	return store.messages.Client.Update(ctx, request)
}

// SaveChat persists chat metadata.
func (store store) SaveChat(ctx context.Context, request chat.UpdateExpression) error {
	return store.chats.Client.Update(ctx, request)
}

// Update stores a message row in DynamoDB.
func (client messageClient) Update(ctx context.Context, request message.UpdateExpression) error {
	return updateItem(ctx, client.dynamo, updateRequest{
		ExpressionAttributeNames:  request.ExpressionAttributeNames,
		ExpressionAttributeValues: request.ExpressionAttributeValues,
		Key:                       request.Key,
		TableName:                 request.TableName,
		UpdateExpression:          request.UpdateExpression,
	})
}

// QueryByChat loads message rows for one chat.
func (client messageClient) QueryByChat(ctx context.Context, request message.QueryRequest) ([]message.Message, error) {
	rows := make([]message.Message, 0)
	startKey := map[string]ddbtypes.AttributeValue(nil)

	for {
		output, err := client.dynamo.Query(ctx, &awsdynamodb.QueryInput{
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
func (client chatClient) Get(ctx context.Context, tableName string, chatID int64) (chat.Row, bool, error) {
	output, err := client.dynamo.GetItem(ctx, &awsdynamodb.GetItemInput{
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
func (client chatClient) Update(ctx context.Context, request chat.UpdateExpression) error {
	return updateItem(ctx, client.dynamo, updateRequest{
		ExpressionAttributeNames:  request.ExpressionAttributeNames,
		ExpressionAttributeValues: request.ExpressionAttributeValues,
		Key:                       request.Key,
		TableName:                 request.TableName,
		UpdateExpression:          request.UpdateExpression,
	})
}

// ListEnabled scans chat settings rows for scheduling.
func (client chatClient) ListEnabled(ctx context.Context, tableName string) ([]chat.Row, error) {
	rows := make([]chat.Row, 0)
	startKey := map[string]ddbtypes.AttributeValue(nil)

	for {
		output, err := client.dynamo.Scan(ctx, &awsdynamodb.ScanInput{
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

// SendMessage sends a queue action to SQS.
func (client sqsClient) SendMessage(ctx context.Context, request queue.SendMessageRequest) error {
	_, err := client.queue.SendMessage(ctx, &awssqs.SendMessageInput{
		QueueUrl:          awscore.String(request.QueueURL),
		MessageBody:       awscore.String(request.MessageBody),
		MessageAttributes: encodeQueueAttributes(request.MessageAttributes),
	})
	if err != nil {
		return fmt.Errorf("send SQS message: %w", err)
	}

	return nil
}

// ReceiveMessage polls one SQS batch.
func (client sqsClient) ReceiveMessage(ctx context.Context, request queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error) {
	maxMessages, err := toInt32(request.MaxNumberOfMessages, "maxNumberOfMessages")
	if err != nil {
		return queue.ReceiveMessageResponse{}, err
	}
	waitSeconds, err := toInt32(request.WaitTimeSeconds, "waitTimeSeconds")
	if err != nil {
		return queue.ReceiveMessageResponse{}, err
	}

	output, err := client.queue.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
		QueueUrl:              awscore.String(request.QueueURL),
		MaxNumberOfMessages:   maxMessages,
		MessageAttributeNames: []string{"All"},
		WaitTimeSeconds:       waitSeconds,
	})
	if err != nil {
		return queue.ReceiveMessageResponse{}, fmt.Errorf("receive SQS messages: %w", err)
	}

	messages := make([]queue.RawMessage, 0, len(output.Messages))
	for _, item := range output.Messages {
		payload, marshalErr := json.Marshal(awscore.ToString(item.Body))
		if marshalErr != nil {
			return queue.ReceiveMessageResponse{}, fmt.Errorf("encode SQS message body: %w", marshalErr)
		}
		messages = append(messages, queue.RawMessage{
			Body:              payload,
			ReceiptHandle:     awscore.ToString(item.ReceiptHandle),
			MessageAttributes: decodeQueueAttributes(item.MessageAttributes),
		})
	}

	return queue.ReceiveMessageResponse{Messages: messages}, nil
}

// Delete removes a consumed SQS message.
func (client sqsClient) Delete(ctx context.Context, request queue.DeleteMessageRequest) error {
	_, err := client.queue.DeleteMessage(ctx, &awssqs.DeleteMessageInput{
		QueueUrl:      awscore.String(request.QueueURL),
		ReceiptHandle: awscore.String(request.ReceiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete SQS message: %w", err)
	}

	return nil
}

// ScaleUp raises DynamoDB read capacity to the configured target.
func (service scaleUpService) ScaleUp(ctx context.Context) error {
	output, err := service.dynamo.DescribeTable(ctx, &awsdynamodb.DescribeTableInput{
		TableName: awscore.String(service.tableName),
	})
	if err != nil {
		return fmt.Errorf("describe DynamoDB table: %w", err)
	}

	throughput := output.Table.ProvisionedThroughput
	request := contractdynamodb.BuildScaleUpRequest(
		service.tableName,
		awscore.ToInt64(throughput.ReadCapacityUnits),
		awscore.ToInt64(throughput.WriteCapacityUnits),
		strconv.Itoa(service.desiredRead),
	)

	_, err = service.dynamo.UpdateTable(ctx, &awsdynamodb.UpdateTableInput{
		TableName: awscore.String(request.TableName),
		ProvisionedThroughput: &ddbtypes.ProvisionedThroughput{
			ReadCapacityUnits:  awscore.Int64(request.ProvisionedThroughput.ReadCapacityUnits),
			WriteCapacityUnits: awscore.Int64(request.ProvisionedThroughput.WriteCapacityUnits),
		},
	})
	if err != nil {
		if contractdynamodb.IsIgnoredScaleUpError(err) {
			return nil
		}
		return fmt.Errorf("update DynamoDB table: %w", err)
	}

	return nil
}

// sendJungHelp sends the bot help response.
func (service actionService) sendJungHelp(ctx context.Context, chatID int64, chatTitle string) error {
	return service.telegramClient.SendMessageWithOptions(ctx, chatID, statistics.HelpMessage(chatTitle), telegram.SendMessageOptions{
		DisableWebPagePreview: true,
		ParseMode:             "markdown",
	})
}

// sendTopTen sends the top-ten report.
func (service actionService) sendTopTen(ctx context.Context, chatID int64) error {
	return service.sendStatistics(ctx, chatID, statistics.Options{Limit: 10})
}

// sendTopDiver sends the reverse ranking report.
func (service actionService) sendTopDiver(ctx context.Context, chatID int64) error {
	return service.sendStatistics(ctx, chatID, statistics.Options{Limit: 10, Reverse: true})
}

// sendAllJung sends the full report when enabled.
func (service actionService) sendAllJung(ctx context.Context, chatID int64) error {
	row, ok, err := service.chatClient.Get(ctx, service.chatTable, chatID)
	if err != nil {
		return err
	}
	if ok && row.EnableAllJung != nil && !*row.EnableAllJung {
		return nil
	}

	return service.sendStatistics(ctx, chatID, statistics.Options{})
}

// sendOffFromWork sends the off-work report.
func (service actionService) sendOffFromWork(ctx context.Context, chatID int64) error {
	return service.sendStatistics(ctx, chatID, statistics.Options{Limit: 10, OffFromWork: true})
}

// enableAllJung updates and replies to the enable command.
func (service actionService) enableAllJung(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
	isAdmin, err := service.telegramClient.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return err
	}

	change := schedule.EnableAllJung(service.chatTable, chatID, chatTitle, isAdmin)
	return service.applySettingChange(ctx, chatID, change)
}

// disableAllJung updates and replies to the disable command.
func (service actionService) disableAllJung(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
	isAdmin, err := service.telegramClient.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return err
	}

	change := schedule.DisableAllJung(service.chatTable, chatID, chatTitle, isAdmin)
	return service.applySettingChange(ctx, chatID, change)
}

// setOffWorkTime updates and replies to the off-work settings command.
func (service actionService) setOffWorkTime(ctx context.Context, input worker.SetOffInput) error {
	isAdmin, err := service.telegramClient.IsAdmin(ctx, input.ChatID, input.UserID)
	if err != nil {
		return err
	}

	change, err := schedule.SetOffFromWorkTimeUTC(service.chatTable, input.ChatID, input.ChatTitle, isAdmin, input.OffTime, input.Workday)
	if err != nil {
		return err
	}

	return service.applySettingChange(ctx, input.ChatID, change)
}

// onOffFromWork fans out due off-work actions for one scheduled instant.
func (service actionService) onOffFromWork(ctx context.Context, timeString string) error {
	timestamp, err := parseScheduledTime(timeString)
	if err != nil {
		return err
	}

	chatIDs, err := service.dueChatIDs(ctx, timestamp)
	if err != nil {
		return err
	}
	producer := queue.Producer{
		QueueURL: service.queueURL,
		Sender:   service.sender,
	}
	for _, chatID := range chatIDs {
		if err := producer.Enqueue(ctx, schedule.BuildOffFromWorkAction(chatID)); err != nil {
			return fmt.Errorf("enqueue due off-work report: %w", err)
		}
		if err := pauseFanOut(ctx, 5*time.Millisecond); err != nil {
			return err
		}
	}

	return nil
}

// buildComponents assembles runtime dependencies from concrete clients.
func buildComponents(serviceConfig config.Config, dynamoClient dynamoAPI, queueClient sqsAPI, telegramClient telegramAPI) Components {
	chatStoreClient := chatClient{dynamo: dynamoClient}
	messageStoreClient := messageClient{dynamo: dynamoClient}
	chatRepository := chat.Repository{
		TableName: serviceConfig.ChatIDTable,
		Client:    chatStoreClient,
	}
	messageRepository := message.Repository{
		TableName: serviceConfig.MessageTable,
		Client:    messageStoreClient,
	}
	queueClientAdapter := sqsClient{queue: queueClient}
	actions := actionService{
		chatClient:         chatStoreClient,
		chatRepository:     chatRepository,
		chatTable:          serviceConfig.ChatIDTable,
		dynamo:             dynamoClient,
		messageRepository:  messageRepository,
		now:                time.Now,
		telegramClient:     telegramClient,
		updateCountsClient: dynamoClient,
		sender:             queueClientAdapter,
		queueURL:           serviceConfig.EventQueueURL,
	}

	return Components{
		Store: store{
			messages: messageRepository,
			chats:    chatRepository,
		},
		Sender:     queueClientAdapter,
		Receiver:   queueClientAdapter,
		Deleter:    queueClientAdapter,
		Messenger:  telegramClient,
		ScaleUpper: scaleUpService{dynamo: dynamoClient, desiredRead: serviceConfig.ScaleUpReadCapacity, tableName: serviceConfig.MessageTable},
		Handlers: worker.Handlers{
			JungHelp:       actions.sendJungHelp,
			TopTen:         actions.sendTopTen,
			TopDiver:       actions.sendTopDiver,
			AllJung:        actions.sendAllJung,
			OffFromWork:    actions.sendOffFromWork,
			EnableAllJung:  actions.enableAllJung,
			DisableAllJung: actions.disableAllJung,
			SetOffWorkTime: actions.setOffWorkTime,
			OnOffFromWork:  actions.onOffFromWork,
		},
		Now: time.Now,
	}
}

// loadAWSConfig builds the shared AWS SDK configuration.
func loadAWSConfig(ctx context.Context, serviceConfig config.Config) (awscore.Config, error) {
	options := []func(*awscfg.LoadOptions) error{
		awscfg.WithRegion(serviceConfig.AWSRegion),
	}

	awsConfig, err := awscfg.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return awscore.Config{}, err
	}

	return awsConfig, nil
}

// dynamoOptions applies DynamoDB client overrides from service configuration.
func dynamoOptions(serviceConfig config.Config) []func(*awsdynamodb.Options) {
	if serviceConfig.AWSEndpointURL == "" {
		return nil
	}

	return []func(*awsdynamodb.Options){
		func(options *awsdynamodb.Options) {
			options.BaseEndpoint = awscore.String(serviceConfig.AWSEndpointURL)
		},
	}
}

// sqsOptions applies SQS client overrides from service configuration.
func sqsOptions(serviceConfig config.Config) []func(*awssqs.Options) {
	if serviceConfig.AWSEndpointURL == "" {
		return nil
	}

	return []func(*awssqs.Options){
		func(options *awssqs.Options) {
			options.BaseEndpoint = awscore.String(serviceConfig.AWSEndpointURL)
		},
	}
}

// updateItem applies a contract update expression in DynamoDB.
func updateItem(ctx context.Context, dynamoClient dynamoAPI, request updateRequest) error {
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

// sendStatistics renders, stores counts, and sends one report.
func (service actionService) sendStatistics(ctx context.Context, chatID int64, options statistics.Options) error {
	now := service.now()
	options.Now = now
	rows, err := service.messageRepository.QueryByChat(ctx, chatID, now.AddDate(0, 0, -7), now)
	if err != nil {
		return err
	}

	summary, err := statistics.GenerateReport(rows, options)
	if err != nil {
		return err
	}
	update := contractdynamodb.BuildChatCountUpdate(service.chatTable, chatID, summary.UserCount, summary.MessageCount, now)
	if err := updateItem(ctx, service.updateCountsClient, updateRequest{
		ExpressionAttributeNames:  update.ExpressionAttributeNames,
		ExpressionAttributeValues: update.ExpressionAttributeValues,
		Key:                       update.Key,
		TableName:                 update.TableName,
		UpdateExpression:          update.UpdateExpression,
	}); err != nil {
		return err
	}

	if err := service.telegramClient.SendMessage(ctx, chatID, summary.Report); err != nil {
		if isTelegramStatusError(err) {
			return nil
		}
		return err
	}

	return nil
}

// dueChatIDs scans and filters due chats for one scheduled window.
func (service actionService) dueChatIDs(ctx context.Context, timestamp time.Time) ([]int64, error) {
	window := schedule.WindowFromTime(timestamp)
	request := contractdynamodb.ScanDueChatsRequest(service.chatTable, window.OffTime, window.Weekday, nil)
	startKey := map[string]ddbtypes.AttributeValue(nil)
	chatIDs := make([]int64, 0)

	for {
		output, err := service.dynamo.Scan(ctx, &awsdynamodb.ScanInput{
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

// applySettingChange writes and replies to one admin settings change.
func (service actionService) applySettingChange(ctx context.Context, chatID int64, change schedule.SettingChange) error {
	if !change.Allowed {
		return nil
	}
	if err := service.chatClient.Update(ctx, change.Update); err != nil {
		return err
	}

	return service.telegramClient.SendMessage(ctx, chatID, change.Reply)
}

// parseScheduledTime parses the scheduler time string.
func parseScheduledTime(raw string) (time.Time, error) {
	if timestamp, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return timestamp, nil
	}

	timestamp, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse scheduled time: %w", err)
	}

	return timestamp, nil
}

// encodeQueueAttributes converts queue attributes for SQS.
func encodeQueueAttributes(attributes map[string]queue.SendMessageAttribute) map[string]sqstypes.MessageAttributeValue {
	encoded := make(map[string]sqstypes.MessageAttributeValue, len(attributes))
	for name, attribute := range attributes {
		encoded[name] = sqstypes.MessageAttributeValue{
			DataType:    awscore.String(attribute.DataType),
			StringValue: awscore.String(attribute.StringValue),
		}
	}

	return encoded
}

// decodeQueueAttributes converts queue attributes from SQS.
func decodeQueueAttributes(attributes map[string]sqstypes.MessageAttributeValue) map[string]queue.MessageAttribute {
	decoded := make(map[string]queue.MessageAttribute, len(attributes))
	for name, attribute := range attributes {
		decoded[name] = queue.MessageAttribute{StringValue: awscore.ToString(attribute.StringValue)}
	}

	return decoded
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

// pauseFanOut preserves the reference scheduler pacing between sends.
func pauseFanOut(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// isTelegramStatusError reports whether err is a Telegram API 4xx or 5xx error.
func isTelegramStatusError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "telegram API returned HTTP 4") ||
		strings.Contains(err.Error(), "telegram API returned HTTP 5")
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

// toInt32 converts an int to int32 with bounds checking for AWS SDK inputs.
func toInt32(value int, field string) (int32, error) {
	if value < -2147483648 || value > 2147483647 {
		return 0, fmt.Errorf("%s out of int32 range", field)
	}

	return int32(value), nil
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
