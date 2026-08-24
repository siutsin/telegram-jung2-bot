// Package dynamodb adapts the internal storage contracts to DynamoDB.
package dynamodb

import (
	"context"
	"time"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=client.go -destination=../mock/dynamodb_mock.go -package=mock -mock_names dynamoRequester=MockDynamoRequester,dependencyObserver=MockDynamoDependencyObserver,scaleUpObserver=MockDynamoScaleUpObserver"

// dynamoRequester is the DynamoDB SDK surface used by the adapters.
type dynamoRequester interface {
	DescribeTable(ctx context.Context, params *awsdynamodb.DescribeTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.DescribeTableOutput, error)
	GetItem(ctx context.Context, params *awsdynamodb.GetItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.GetItemOutput, error)
	Query(ctx context.Context, params *awsdynamodb.QueryInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.QueryOutput, error)
	Scan(ctx context.Context, params *awsdynamodb.ScanInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.ScanOutput, error)
	UpdateItem(ctx context.Context, params *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error)
	UpdateTable(ctx context.Context, params *awsdynamodb.UpdateTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateTableOutput, error)
}

// dependencyObserver records a completed DynamoDB request.
type dependencyObserver interface {
	ObserveDependency(dependency string, operation string, duration time.Duration, err error)
}

// scaleUpObserver records DynamoDB requests and scale-up results.
type scaleUpObserver interface {
	dependencyObserver
	RecordScaleUp(result string)
}

// MessageClient is the DynamoDB-backed message adapter.
type MessageClient struct {
	dynamo  dynamoRequester
	metrics dependencyObserver
}

// ChatClient is the DynamoDB-backed chat adapter.
type ChatClient struct {
	dynamo  dynamoRequester
	metrics dependencyObserver
}

// ScaleUpper is the DynamoDB-backed scale-up adapter.
type ScaleUpper struct {
	dynamo      dynamoRequester
	desiredRead int
	tableName   string
	metrics     scaleUpObserver
}

// NewMessageClient builds the DynamoDB-backed message adapter.
func NewMessageClient(dynamoClient dynamoRequester, metrics ...dependencyObserver) MessageClient {
	return MessageClient{dynamo: dynamoClient, metrics: firstMetrics(metrics)}
}

// NewChatClient builds the DynamoDB-backed chat adapter.
func NewChatClient(dynamoClient dynamoRequester, metrics ...dependencyObserver) ChatClient {
	return ChatClient{dynamo: dynamoClient, metrics: firstMetrics(metrics)}
}

// NewScaleUpper builds the DynamoDB-backed scale-up adapter.
func NewScaleUpper(dynamoClient dynamoRequester, tableName string, desiredRead int, metrics ...scaleUpObserver) ScaleUpper {
	return ScaleUpper{
		dynamo:      dynamoClient,
		desiredRead: desiredRead,
		tableName:   tableName,
		metrics:     firstScaleUpObserver(metrics),
	}
}

// firstMetrics returns the optional dependency observer from application wiring.
func firstMetrics(metrics []dependencyObserver) dependencyObserver {
	if len(metrics) == 0 {
		return nil
	}

	return metrics[0]
}

// firstScaleUpObserver returns the optional scale-up observer from application wiring.
func firstScaleUpObserver(metrics []scaleUpObserver) scaleUpObserver {
	if len(metrics) == 0 {
		return nil
	}

	return metrics[0]
}

// observeDependency records a completed DynamoDB request when metrics are enabled.
func observeDependency(metrics dependencyObserver, operation string, started time.Time, err error) {
	if metrics != nil {
		metrics.ObserveDependency("dynamodb", operation, time.Since(started), err)
	}
}
