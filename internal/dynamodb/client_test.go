package dynamodb

import (
	"context"
	"testing"
	"time"

	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	mock "github.com/siutsin/telegram-jung2-bot/internal/mock"
)

func TestNewMessageClient(t *testing.T) {
	t.Parallel()

	dynamoClient := mock.NewMockDynamoRequester(gomock.NewController(t))

	client := NewMessageClient(dynamoClient)

	assert.Same(t, dynamoClient, client.dynamo)
}

func TestNewChatClient(t *testing.T) {
	t.Parallel()

	dynamoClient := mock.NewMockDynamoRequester(gomock.NewController(t))

	client := NewChatClient(dynamoClient)

	assert.Same(t, dynamoClient, client.dynamo)
}

func TestNewScaleUpper(t *testing.T) {
	t.Parallel()

	dynamoClient := mock.NewMockDynamoRequester(gomock.NewController(t))

	scaleUpper := NewScaleUpper(dynamoClient, "messages", 10)

	assert.Same(t, dynamoClient, scaleUpper.dynamo)
	assert.Equal(t, "messages", scaleUpper.tableName)
	assert.Equal(t, 10, scaleUpper.desiredRead)
}

// TestMessageClientRecordsDynamoDBMetrics proves a real query notifies the generated observer.
func TestMessageClientRecordsDynamoDBMetrics(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	dynamoClient := mock.NewMockDynamoRequester(controller)
	metrics := mock.NewMockDynamoDependencyObserver(controller)
	dynamoClient.EXPECT().
		Query(gomock.Any(), gomock.Any()).
		Return(&awsdynamodb.QueryOutput{}, nil)
	metrics.EXPECT().ObserveDependency("dynamodb", "query", gomock.Any(), nil)

	messageClient := NewMessageClient(dynamoClient, metrics)
	_, err := messageClient.QueryByChat(context.Background(), "messages", 123, time.Time{})

	require.NoError(t, err)
	assert.Same(t, metrics, messageClient.metrics)
}
