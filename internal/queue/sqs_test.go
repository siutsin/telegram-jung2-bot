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
	action := DecodeMessage(response.Messages[0])
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
			assert.Equal(t, "42", awscore.ToString(input.MessageGroupId))
			assert.Equal(t, "42:7", awscore.ToString(input.MessageDeduplicationId))

			return &awssqs.SendMessageOutput{}, nil
		})

	client := NewClient(queueAPI)
	err := client.SendMessage(context.Background(), SendMessageRequest{
		QueueURL:    "queue-url",
		MessageBody: BodyTopTen,
		MessageAttributes: map[string]SendMessageAttribute{
			"action": {DataType: "String", StringValue: ActionTopTen},
		},
		MessageGroupID:         "42",
		MessageDeduplicationID: "42:7",
	})

	require.NoError(t, err)
}

// TestClientSendMessageBatchPreservesFIFOFields keeps FIFO grouping and deduplication in the AWS request.
func TestClientSendMessageBatchPreservesFIFOFields(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	queueAPI := mock.NewMockQueueRequester(controller)
	queueAPI.EXPECT().
		SendMessageBatch(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *awssqs.SendMessageBatchInput, optFns ...func(*awssqs.Options)) (*awssqs.SendMessageBatchOutput, error) {
			require.Len(t, input.Entries, 1)
			assert.Equal(t, "42", awscore.ToString(input.Entries[0].MessageGroupId))
			assert.Equal(t, "42:7", awscore.ToString(input.Entries[0].MessageDeduplicationId))
			return &awssqs.SendMessageBatchOutput{}, nil
		})

	err := NewClient(queueAPI).SendMessageBatch(context.Background(), SendMessageBatchRequest{
		QueueURL: "fifo-url",
		Entries: []SendMessageBatchEntry{{
			ID:                     "0",
			MessageBody:            BodySaveMessage,
			MessageGroupID:         "42",
			MessageDeduplicationID: "42:7",
		}},
	})

	require.NoError(t, err)
}

// TestClientDeleteBatchRemovesReceivedMessages keeps receive and delete request costs batched.
func TestClientDeleteBatchRemovesReceivedMessages(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	queueAPI := mock.NewMockQueueRequester(controller)
	queueAPI.EXPECT().
		DeleteMessageBatch(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *awssqs.DeleteMessageBatchInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageBatchOutput, error) {
			require.Len(t, input.Entries, 2)
			assert.Equal(t, "one", awscore.ToString(input.Entries[0].ReceiptHandle))
			return &awssqs.DeleteMessageBatchOutput{}, nil
		})

	err := NewClient(queueAPI).DeleteBatch(context.Background(), DeleteMessageBatchRequest{
		QueueURL:       "fifo-url",
		ReceiptHandles: []string{"one", "two"},
	})

	require.NoError(t, err)
}

// TestClientBatchOperationsReturnSQSFailures keeps a partial batch from being acknowledged.
func TestClientBatchOperationsReturnSQSFailures(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	queueAPI := mock.NewMockQueueRequester(controller)
	gomock.InOrder(
		queueAPI.EXPECT().DeleteMessageBatch(gomock.Any(), gomock.Any()).Return(&awssqs.DeleteMessageBatchOutput{Failed: []sqstypes.BatchResultErrorEntry{{}}}, nil),
		queueAPI.EXPECT().DeleteMessageBatch(gomock.Any(), gomock.Any()).Return(nil, errors.New("delete down")),
		queueAPI.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).Return(nil, errors.New("send down")),
		queueAPI.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).Return(&awssqs.SendMessageBatchOutput{Failed: []sqstypes.BatchResultErrorEntry{{}}}, nil),
	)
	client := NewClient(queueAPI)

	err := client.DeleteBatch(context.Background(), DeleteMessageBatchRequest{ReceiptHandles: []string{"one"}})
	require.EqualError(t, err, "delete SQS message batch: 1 entries failed")
	err = client.DeleteBatch(context.Background(), DeleteMessageBatchRequest{})
	require.EqualError(t, err, "delete SQS message batch: delete down")
	err = client.SendMessageBatch(context.Background(), SendMessageBatchRequest{})
	require.EqualError(t, err, "send SQS message batch: send down")
	err = client.SendMessageBatch(context.Background(), SendMessageBatchRequest{})
	require.EqualError(t, err, "send SQS message batch: 1 entries failed")
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
		{
			name: "delete batch",
			run: func() error {
				return client.DeleteBatch(context.Background(), DeleteMessageBatchRequest{})
			},
		},
		{
			name: "send batch",
			run: func() error {
				return client.SendMessageBatch(context.Background(), SendMessageBatchRequest{})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.EqualError(t, test.run(), "queue client is required")
		})
	}
}

// TestClientRecordsSQSMetric proves a real SQS send notifies the generated observer.
func TestClientRecordsSQSMetric(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	queueAPI := mock.NewMockQueueRequester(controller)
	metrics := mock.NewMockQueueDependencyObserver(controller)
	queueAPI.EXPECT().
		SendMessage(gomock.Any(), gomock.Any()).
		Return(&awssqs.SendMessageOutput{}, nil)
	metrics.EXPECT().ObserveDependency("sqs", "send", gomock.Any(), nil)
	client := NewClient(queueAPI, metrics)
	err := client.SendMessage(context.Background(), SendMessageRequest{})

	require.NoError(t, err)
	assert.Same(t, metrics, client.metrics)
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
