package integration

import (
	"context"
	"fmt"
	"strconv"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

type awsClients struct {
	dynamo *awsdynamodb.Client
	sqs    *awssqs.Client
}

type testResources struct {
	messageTable string
	chatTable    string
	queueName    string
	queueURL     string
}

func newAWSClients(ctx context.Context, endpoint string, region string) (awsClients, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(awscredentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return awsClients{}, fmt.Errorf("load AWS config: %w", err)
	}

	return awsClients{
		dynamo: awsdynamodb.NewFromConfig(cfg, func(options *awsdynamodb.Options) {
			options.BaseEndpoint = awscore.String(endpoint)
		}),
		sqs: awssqs.NewFromConfig(cfg, func(options *awssqs.Options) {
			options.BaseEndpoint = awscore.String(endpoint)
		}),
	}, nil
}

func provisionResources(ctx context.Context, clients awsClients) (testResources, func(), error) {
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	resources := testResources{
		messageTable: "telegram-jung2-bot-messages-it-" + suffix,
		chatTable:    "telegram-jung2-bot-chats-it-" + suffix,
		queueName:    "telegram-jung2-bot-it-" + suffix,
	}
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cleanupResources(cleanupCtx, clients, resources)
	}

	err := createDynamoTable(ctx, clients.dynamo, resources.messageTable, []ddbtypes.AttributeDefinition{
		{AttributeName: awscore.String("chatId"), AttributeType: ddbtypes.ScalarAttributeTypeN},
		{AttributeName: awscore.String("dateCreated"), AttributeType: ddbtypes.ScalarAttributeTypeS},
	}, []ddbtypes.KeySchemaElement{
		{AttributeName: awscore.String("chatId"), KeyType: ddbtypes.KeyTypeHash},
		{AttributeName: awscore.String("dateCreated"), KeyType: ddbtypes.KeyTypeRange},
	})
	if err != nil {
		return resources, cleanup, err
	}

	err = createDynamoTable(ctx, clients.dynamo, resources.chatTable, []ddbtypes.AttributeDefinition{
		{AttributeName: awscore.String("chatId"), AttributeType: ddbtypes.ScalarAttributeTypeN},
	}, []ddbtypes.KeySchemaElement{
		{AttributeName: awscore.String("chatId"), KeyType: ddbtypes.KeyTypeHash},
	})
	if err != nil {
		return resources, cleanup, err
	}

	output, err := clients.sqs.CreateQueue(ctx, &awssqs.CreateQueueInput{
		QueueName: awscore.String(resources.queueName),
	})
	if err != nil {
		return resources, cleanup, fmt.Errorf("create SQS queue: %w", err)
	}
	resources.queueURL = awscore.ToString(output.QueueUrl)

	return resources, cleanup, nil
}

func createDynamoTable(
	ctx context.Context,
	client *awsdynamodb.Client,
	tableName string,
	attributes []ddbtypes.AttributeDefinition,
	keys []ddbtypes.KeySchemaElement,
) error {
	_, err := client.CreateTable(ctx, &awsdynamodb.CreateTableInput{
		TableName:            awscore.String(tableName),
		AttributeDefinitions: attributes,
		KeySchema:            keys,
		BillingMode:          ddbtypes.BillingModePayPerRequest,
	})
	if err != nil {
		return fmt.Errorf("create DynamoDB table %s: %w", tableName, err)
	}

	return waitForTableActive(ctx, client, tableName)
}

func waitForTableActive(ctx context.Context, client *awsdynamodb.Client, tableName string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		output, err := client.DescribeTable(ctx, &awsdynamodb.DescribeTableInput{
			TableName: awscore.String(tableName),
		})
		if err != nil {
			return fmt.Errorf("describe DynamoDB table %s: %w", tableName, err)
		}
		if output.Table != nil && output.Table.TableStatus == ddbtypes.TableStatusActive {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for DynamoDB table %s: %w", tableName, ctx.Err())
		case <-ticker.C:
		}
	}
}

func cleanupResources(ctx context.Context, clients awsClients, resources testResources) {
	if resources.queueURL != "" {
		_, err := clients.sqs.DeleteQueue(ctx, &awssqs.DeleteQueueInput{
			QueueUrl: awscore.String(resources.queueURL),
		})
		if err != nil {
			reportCleanupError("delete SQS queue", err)
		}
	}
	if resources.messageTable != "" {
		_, err := clients.dynamo.DeleteTable(ctx, &awsdynamodb.DeleteTableInput{
			TableName: awscore.String(resources.messageTable),
		})
		if err != nil {
			reportCleanupError("delete message table", err)
		}
	}
	if resources.chatTable != "" {
		_, err := clients.dynamo.DeleteTable(ctx, &awsdynamodb.DeleteTableInput{
			TableName: awscore.String(resources.chatTable),
		})
		if err != nil {
			reportCleanupError("delete chat table", err)
		}
	}
}
