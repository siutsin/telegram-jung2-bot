package queue

import (
	"context"
	"errors"
	"math"
	"testing"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/siutsin/telegram-jung2-bot/internal/mock"
)

func TestClientReceiveMessageSupportsContractAttributes(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	queueAPI := mock.NewMockQueueRequester(controller)
	queueAPI.EXPECT().
		ReceiveMessage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
			assert.Equal(t, "queue-url", awscore.ToString(input.QueueUrl))
			assert.Equal(t, int32(10), input.MaxNumberOfMessages)
			assert.Equal(t, int32(20), input.WaitTimeSeconds)
			assert.Equal(t, []string{"All"}, input.MessageAttributeNames)

			return &awssqs.ReceiveMessageOutput{
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
			}, nil
		})
	client := NewClient(queueAPI)
	response, err := client.ReceiveMessage(context.Background(), ReceiveMessageRequest{
		MaxNumberOfMessages: 10,
		QueueURL:            "queue-url",
		WaitTimeSeconds:     20,
	})

	require.NoError(t, err)
	require.Len(t, response.Messages, 1)
	assert.Equal(t, "receipt", response.Messages[0].ReceiptHandle)
	assert.Equal(t, `"sendTopTenMessage"`, string(response.Messages[0].Body))
	action, err := DecodeMessage(response.Messages[0])
	require.NoError(t, err)
	assert.Equal(t, ActionTopTen, action.Attributes["action"])
}

func TestClientSendMessageEncodesAttributes(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	queueAPI := mock.NewMockQueueRequester(controller)
	queueAPI.EXPECT().
		SendMessage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *awssqs.SendMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error) {
			assert.Equal(t, "queue-url", awscore.ToString(input.QueueUrl))
			assert.Equal(t, BodyTopTen, awscore.ToString(input.MessageBody))
			assert.Equal(t, "String", awscore.ToString(input.MessageAttributes["action"].DataType))
			assert.Equal(t, ActionTopTen, awscore.ToString(input.MessageAttributes["action"].StringValue))

			return &awssqs.SendMessageOutput{}, nil
		})

	client := NewClient(queueAPI)
	err := client.SendMessage(context.Background(), SendMessageRequest{
		QueueURL:    "queue-url",
		MessageBody: BodyTopTen,
		MessageAttributes: map[string]SendMessageAttribute{
			"action": {DataType: "String", StringValue: ActionTopTen},
		},
	})

	require.NoError(t, err)
}

func TestClientDeleteRemovesConsumedMessage(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	queueAPI := mock.NewMockQueueRequester(controller)
	queueAPI.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
			assert.Equal(t, "queue-url", awscore.ToString(input.QueueUrl))
			assert.Equal(t, "receipt", awscore.ToString(input.ReceiptHandle))

			return &awssqs.DeleteMessageOutput{}, nil
		})

	client := NewClient(queueAPI)
	err := client.Delete(context.Background(), DeleteMessageRequest{
		QueueURL:      "queue-url",
		ReceiptHandle: "receipt",
	})

	require.NoError(t, err)
}

func TestClientRequiresQueue(t *testing.T) {
	t.Parallel()

	client := NewClient(nil)
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "receive",
			run: func() error {
				response, err := client.ReceiveMessage(context.Background(), ReceiveMessageRequest{})
				assert.Equal(t, ReceiveMessageResponse{}, response)
				return err
			},
		},
		{
			name: "delete",
			run: func() error {
				return client.Delete(context.Background(), DeleteMessageRequest{})
			},
		},
		{
			name: "send",
			run: func() error {
				return client.SendMessage(context.Background(), SendMessageRequest{})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.EqualError(t, test.run(), "queue client is required")
		})
	}
}

func TestClientWrapsQueueErrors(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	queueAPI := mock.NewMockQueueRequester(controller)
	queueErr := errors.New("boom")
	queueAPI.EXPECT().
		ReceiveMessage(gomock.Any(), gomock.Any()).
		Return(nil, queueErr)
	queueAPI.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		Return(nil, queueErr)
	queueAPI.EXPECT().
		SendMessage(gomock.Any(), gomock.Any()).
		Return(nil, queueErr)

	client := NewClient(queueAPI)
	tests := []struct {
		name    string
		run     func() error
		wantErr string
	}{
		{
			name: "receive",
			run: func() error {
				_, err := client.ReceiveMessage(context.Background(), ReceiveMessageRequest{})
				return err
			},
			wantErr: "receive SQS messages: boom",
		},
		{
			name: "delete",
			run: func() error {
				return client.Delete(context.Background(), DeleteMessageRequest{})
			},
			wantErr: "delete SQS message: boom",
		},
		{
			name: "send",
			run: func() error {
				return client.SendMessage(context.Background(), SendMessageRequest{})
			},
			wantErr: "send SQS message: boom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.EqualError(t, test.run(), test.wantErr)
		})
	}
}

func TestClientReceiveMessageRejectsOutOfRangeOptions(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	client := NewClient(mock.NewMockQueueRequester(controller))
	tests := []struct {
		name    string
		request ReceiveMessageRequest
		wantErr string
	}{
		{
			name:    "max number too large",
			request: ReceiveMessageRequest{MaxNumberOfMessages: math.MaxInt32 + 1},
			wantErr: "maxNumberOfMessages out of int32 range",
		},
		{
			name:    "wait time too large",
			request: ReceiveMessageRequest{WaitTimeSeconds: math.MaxInt32 + 1},
			wantErr: "waitTimeSeconds out of int32 range",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.ReceiveMessage(context.Background(), test.request)

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
}
