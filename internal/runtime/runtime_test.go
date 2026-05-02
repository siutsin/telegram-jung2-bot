package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildComponentsHelpHandlerSendsMarkdownHelp(t *testing.T) {
	t.Parallel()

	telegramClient := &fakeTelegramClient{}
	components := buildComponents(runtimeConfig(), &fakeDynamoClient{}, &fakeSQSClient{}, telegramClient)

	err := components.Handlers.JungHelp(context.Background(), 123, "Group")

	require.NoError(t, err)
	assert.Equal(t, int64(123), telegramClient.chatID)
	assert.Equal(t, statisticsHelpMessage("Group"), telegramClient.text)
	assert.Equal(t, telegram.SendMessageOptions{
		DisableWebPagePreview: true,
		ParseMode:             "markdown",
	}, telegramClient.options)
}

func TestBuildComponentsWiresScaleUpAndWebhookMessenger(t *testing.T) {
	t.Parallel()

	telegramClient := &fakeTelegramClient{}
	components := buildComponents(runtimeConfig(), &fakeDynamoClient{}, &fakeSQSClient{}, telegramClient)

	require.NotNil(t, components.Messenger)
	require.NotNil(t, components.ScaleUpper)
}

func TestScaleUpServiceIgnoresKnownErrors(t *testing.T) {
	t.Parallel()

	err := (scaleUpService{
		dynamo: &fakeDynamoClient{
			describeOutput: &awsdynamodb.DescribeTableOutput{
				Table: &ddbtypes.TableDescription{
					ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
						ReadCapacityUnits:  awscore.Int64(1),
						WriteCapacityUnits: awscore.Int64(3),
					},
				},
			},
			updateTableErr: errors.New("Subscriber limit exceeded"),
		},
		desiredRead: 10,
		tableName:   "messages",
	}).ScaleUp(context.Background())

	require.NoError(t, err)
}

func TestParseScheduledTimeRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := parseScheduledTime("bad")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse scheduled time")
}

func TestSQSClientReceiveMessageSupportsContractAttributes(t *testing.T) {
	t.Parallel()

	response, err := (sqsClient{queue: &fakeSQSClient{
		receiveOutput: &awssqs.ReceiveMessageOutput{
			Messages: []sqstypes.Message{
				{
					Body:          awscore.String("sendTopTenMessage"),
					ReceiptHandle: awscore.String("receipt"),
					MessageAttributes: map[string]sqstypes.MessageAttributeValue{
						"action": {StringValue: awscore.String("topten")},
						"chatId": {StringValue: awscore.String("123")},
					},
				},
			},
		},
	}}).ReceiveMessage(context.Background(), queue.ReceiveMessageRequest{
		MaxNumberOfMessages: 10,
		QueueURL:            "queue-url",
		WaitTimeSeconds:     20,
	})

	require.NoError(t, err)
	require.Len(t, response.Messages, 1)
	assert.Equal(t, "receipt", response.Messages[0].ReceiptHandle)
	assert.Equal(t, "topten", response.Messages[0].MessageAttributes["action"].StringValue)
}

func TestBuildComponentsOnOffFromWorkHandlerEnqueuesDueChats(t *testing.T) {
	t.Parallel()

	dynamo := &fakeDynamoClient{
		scanOutputs: []*awsdynamodb.ScanOutput{
			{
				Items: []map[string]ddbtypes.AttributeValue{
					{
						"chatId":  &ddbtypes.AttributeValueMemberN{Value: "123"},
						"offTime": &ddbtypes.AttributeValueMemberS{Value: "1800"},
						"workday": &ddbtypes.AttributeValueMemberN{Value: "32"},
					},
				},
			},
		},
	}
	queueClient := &fakeSQSClient{}
	components := buildComponents(runtimeConfig(), dynamo, queueClient, &fakeTelegramClient{})

	err := components.Handlers.OnOffFromWork(context.Background(), "2026-05-01T18:00:00+01:00")

	require.NoError(t, err)
	require.Len(t, queueClient.sendInputs, 1)
	assert.Equal(t, queue.BodyOffFromWork, awscore.ToString(queueClient.sendInputs[0].MessageBody))
	assert.Equal(t, "123", awscore.ToString(queueClient.sendInputs[0].MessageAttributes["chatId"].StringValue))
}

func TestBuildComponentsQueueHelpSlice(t *testing.T) {
	t.Parallel()

	telegramClient := &fakeTelegramClient{}
	queueClient := &fakeSQSClient{}
	components := buildComponents(runtimeConfig(), &fakeDynamoClient{}, queueClient, telegramClient)
	raw := queue.RawMessage{
		ReceiptHandle: "receipt",
		MessageAttributes: map[string]queue.MessageAttribute{
			"action":    {StringValue: queue.ActionJungHelp},
			"chatId":    {StringValue: "123"},
			"chatTitle": {StringValue: "Group"},
		},
	}

	err := worker.ProcessMessage(context.Background(), "https://example.com/queue", raw, components.Handlers, components.Deleter)

	require.NoError(t, err)
	assert.Equal(t, int64(123), telegramClient.chatID)
	assert.Equal(t, statisticsHelpMessage("Group"), telegramClient.text)
	assert.Equal(t, telegram.SendMessageOptions{
		DisableWebPagePreview: true,
		ParseMode:             "markdown",
	}, telegramClient.options)
	require.Len(t, queueClient.deleteInputs, 1)
	assert.Equal(t, "receipt", awscore.ToString(queueClient.deleteInputs[0].ReceiptHandle))
}

func TestBuildComponentsTopTenIgnoresTelegramStatusErrors(t *testing.T) {
	t.Parallel()

	telegramClient := &fakeTelegramClient{err: errors.New("telegram API returned HTTP 403")}
	dynamo := &fakeDynamoClient{
		queryOutputs: []*awsdynamodb.QueryOutput{
			{
				Items: []map[string]ddbtypes.AttributeValue{
					{
						"chatId":      &ddbtypes.AttributeValueMemberN{Value: "123"},
						"chatTitle":   &ddbtypes.AttributeValueMemberS{Value: "Group"},
						"dateCreated": &ddbtypes.AttributeValueMemberS{Value: "2026-05-02T20:00:00+08:00"},
						"firstName":   &ddbtypes.AttributeValueMemberS{Value: "Ada"},
						"ttl":         &ddbtypes.AttributeValueMemberN{Value: "1"},
						"userId":      &ddbtypes.AttributeValueMemberN{Value: "1"},
					},
				},
			},
		},
	}
	components := buildComponents(runtimeConfig(), dynamo, &fakeSQSClient{}, telegramClient)

	err := components.Handlers.TopTen(context.Background(), 123)

	require.NoError(t, err)
	require.NotEmpty(t, dynamo.updateInputs)
}

func statisticsHelpMessage(chatTitle string) string {
	return "\n" +
		"圍爐區: " + chatTitle + "\n\n" +
		"冗員[jung2jyun4] Excess personnel in Cantonese\n\n" +
		"This bot is created for counting the number of message per participant in the group.\n\n" +
		"Commands:\n" +
		"/topTen  show top ten 冗員s\n" +
		"/topDiver  show top ten 潛水員s (潛得太深會搵唔到)\n" +
		"/allJung  show all 冗員s\n" +
		"/jungHelp  show help message\n\n" +
		"Admin Only:\n" +
		"/enableAllJung  enable `/alljung` command\n" +
		"/disableAllJung  disable `/alljung` command\n" +
		"/setOffFromWorkTimeUTC  set offFromWork time (UTC time)\n\n" +
		"[Bug Report/Suggestion](https://github.com/siutsin/telegram-jung2-bot/issues)\n" +
		"[Service Status](https://stats.uptimerobot.com/kglZJSkYZg)\n\n" +
		"May your 冗 power powerful\n"
}

func runtimeConfig() config.Config {
	return config.Config{
		AWSRegion:           "eu-west-1",
		ChatIDTable:         "chats",
		EventQueueURL:       "https://example.com/queue",
		HTTPTimeout:         time.Second,
		MessageTable:        "messages",
		ScaleUpReadCapacity: 10,
		TelegramBotToken:    "token",
		TelegramAPIBaseURL:  "https://api.telegram.org",
	}
}

type fakeTelegramClient struct {
	chatID  int64
	err     error
	text    string
	options telegram.SendMessageOptions
}

func (client *fakeTelegramClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	client.chatID = chatID
	client.text = text
	return client.err
}

func (client *fakeTelegramClient) SendMessageWithOptions(ctx context.Context, chatID int64, text string, options telegram.SendMessageOptions) error {
	client.chatID = chatID
	client.text = text
	client.options = options
	return client.err
}

func (client *fakeTelegramClient) IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error) {
	return true, nil
}

type fakeDynamoClient struct {
	describeOutput *awsdynamodb.DescribeTableOutput
	getOutput      *awsdynamodb.GetItemOutput
	queryOutputs   []*awsdynamodb.QueryOutput
	scanOutputs    []*awsdynamodb.ScanOutput
	updateInputs   []*awsdynamodb.UpdateItemInput
	updateTableErr error
}

func (client *fakeDynamoClient) DescribeTable(ctx context.Context, params *awsdynamodb.DescribeTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.DescribeTableOutput, error) {
	if client.describeOutput != nil {
		return client.describeOutput, nil
	}
	return &awsdynamodb.DescribeTableOutput{
		Table: &ddbtypes.TableDescription{
			ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
				ReadCapacityUnits:  awscore.Int64(1),
				WriteCapacityUnits: awscore.Int64(1),
			},
		},
	}, nil
}

func (client *fakeDynamoClient) GetItem(ctx context.Context, params *awsdynamodb.GetItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.GetItemOutput, error) {
	if client.getOutput != nil {
		return client.getOutput, nil
	}
	return &awsdynamodb.GetItemOutput{}, nil
}

func (client *fakeDynamoClient) Query(ctx context.Context, params *awsdynamodb.QueryInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.QueryOutput, error) {
	if len(client.queryOutputs) == 0 {
		return &awsdynamodb.QueryOutput{}, nil
	}

	output := client.queryOutputs[0]
	client.queryOutputs = client.queryOutputs[1:]
	return output, nil
}

func (client *fakeDynamoClient) Scan(ctx context.Context, params *awsdynamodb.ScanInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.ScanOutput, error) {
	if len(client.scanOutputs) == 0 {
		return &awsdynamodb.ScanOutput{}, nil
	}

	output := client.scanOutputs[0]
	client.scanOutputs = client.scanOutputs[1:]
	return output, nil
}

func (client *fakeDynamoClient) UpdateItem(ctx context.Context, params *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error) {
	client.updateInputs = append(client.updateInputs, params)
	return &awsdynamodb.UpdateItemOutput{}, nil
}

func (client *fakeDynamoClient) UpdateTable(ctx context.Context, params *awsdynamodb.UpdateTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateTableOutput, error) {
	if client.updateTableErr != nil {
		return nil, client.updateTableErr
	}
	return &awsdynamodb.UpdateTableOutput{}, nil
}

type fakeSQSClient struct {
	deleteInputs  []*awssqs.DeleteMessageInput
	receiveOutput *awssqs.ReceiveMessageOutput
	sendInputs    []*awssqs.SendMessageInput
}

func (client *fakeSQSClient) DeleteMessage(ctx context.Context, params *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
	client.deleteInputs = append(client.deleteInputs, params)
	return &awssqs.DeleteMessageOutput{}, nil
}

func (client *fakeSQSClient) ReceiveMessage(ctx context.Context, params *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	if client.receiveOutput != nil {
		return client.receiveOutput, nil
	}
	return &awssqs.ReceiveMessageOutput{}, nil
}

func (client *fakeSQSClient) SendMessage(ctx context.Context, params *awssqs.SendMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error) {
	client.sendInputs = append(client.sendInputs, params)
	return &awssqs.SendMessageOutput{}, nil
}

var _ worker.Deleter = sqsClient{}
