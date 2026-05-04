package dynamodb

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
