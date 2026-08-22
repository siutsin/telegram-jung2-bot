// Package dynamodb adapts the internal storage contracts to DynamoDB.
package dynamodb

import (
	"context"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=client.go -destination=../mock/dynamodb_mock.go -package=mock -mock_names dynamoRequester=MockDynamoRequester"

// dynamoRequester is the DynamoDB SDK surface used by the adapters.
type dynamoRequester interface {
	DescribeTable(ctx context.Context, params *awsdynamodb.DescribeTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.DescribeTableOutput, error)
	GetItem(ctx context.Context, params *awsdynamodb.GetItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.GetItemOutput, error)
	Query(ctx context.Context, params *awsdynamodb.QueryInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.QueryOutput, error)
	Scan(ctx context.Context, params *awsdynamodb.ScanInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.ScanOutput, error)
	UpdateItem(ctx context.Context, params *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error)
	UpdateTable(ctx context.Context, params *awsdynamodb.UpdateTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateTableOutput, error)
}

// MessageClient is the DynamoDB-backed message adapter.
type MessageClient struct {
	dynamo dynamoRequester
}

// ChatClient is the DynamoDB-backed chat adapter.
type ChatClient struct {
	dynamo dynamoRequester
}

// ScaleUpper is the DynamoDB-backed scale-up adapter.
type ScaleUpper struct {
	dynamo      dynamoRequester
	desiredRead int
	tableName   string
}

// NewMessageClient builds the DynamoDB-backed message adapter.
func NewMessageClient(dynamoClient dynamoRequester) MessageClient {
	return MessageClient{dynamo: dynamoClient}
}

// NewChatClient builds the DynamoDB-backed chat adapter.
func NewChatClient(dynamoClient dynamoRequester) ChatClient {
	return ChatClient{dynamo: dynamoClient}
}

// NewScaleUpper builds the DynamoDB-backed scale-up adapter.
func NewScaleUpper(dynamoClient dynamoRequester, tableName string, desiredRead int) ScaleUpper {
	return ScaleUpper{
		dynamo:      dynamoClient,
		desiredRead: desiredRead,
		tableName:   tableName,
	}
}
