package queue

import (
	"context"
	"fmt"
	"strconv"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=sqs.go -destination=../mock/queue_mock.go -package=mock -mock_names queueRequester=MockQueueRequester"

// queueRequester is the SQS SDK surface used by the queue adapter.
type queueRequester interface {
	DeleteMessage(ctx context.Context, params *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error)
	ReceiveMessage(ctx context.Context, params *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error)
	SendMessage(ctx context.Context, params *awssqs.SendMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error)
}

// sqsClient adapts the AWS SQS SDK to the queue package contracts.
type sqsClient struct {
	queue queueRequester
}

// NewClient builds an SQS-backed queue client.
func NewClient(queue queueRequester) sqsClient {
	return sqsClient{queue: queue}
}

// Delete removes a consumed SQS message.
func (client sqsClient) Delete(ctx context.Context, request DeleteMessageRequest) error {
	if client.queue == nil {
		return fmt.Errorf("queue client is required")
	}

	_, err := client.queue.DeleteMessage(ctx, &awssqs.DeleteMessageInput{
		QueueUrl:      awscore.String(request.QueueURL),
		ReceiptHandle: awscore.String(request.ReceiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete SQS message: %w", err)
	}

	return nil
}

// ReceiveMessage polls one SQS batch.
// For example, one AWS message becomes one RawMessage with JSON body text and
// decoded attributes.
func (client sqsClient) ReceiveMessage(ctx context.Context, request ReceiveMessageRequest) (ReceiveMessageResponse, error) {
	if client.queue == nil {
		return ReceiveMessageResponse{}, fmt.Errorf("queue client is required")
	}

	maxMessages, err := toInt32(request.MaxNumberOfMessages, "maxNumberOfMessages")
	if err != nil {
		return ReceiveMessageResponse{}, err
	}
	waitSeconds, err := toInt32(request.WaitTimeSeconds, "waitTimeSeconds")
	if err != nil {
		return ReceiveMessageResponse{}, err
	}

	output, err := client.queue.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
		QueueUrl:              awscore.String(request.QueueURL),
		MaxNumberOfMessages:   maxMessages,
		MessageAttributeNames: []string{"All"},
		WaitTimeSeconds:       waitSeconds,
	})
	if err != nil {
		return ReceiveMessageResponse{}, fmt.Errorf("receive SQS messages: %w", err)
	}

	messages := make([]RawMessage, 0, len(output.Messages))
	for _, item := range output.Messages {
		payload := strconv.Quote(awscore.ToString(item.Body))
		messages = append(messages, RawMessage{
			Body:              []byte(payload),
			ReceiptHandle:     awscore.ToString(item.ReceiptHandle),
			MessageAttributes: decodeQueueAttributes(item.MessageAttributes),
		})
	}

	return ReceiveMessageResponse{Messages: messages}, nil
}

// SendMessage sends a queue action to SQS.
func (client sqsClient) SendMessage(ctx context.Context, request SendMessageRequest) error {
	if client.queue == nil {
		return fmt.Errorf("queue client is required")
	}

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

// encodeQueueAttributes converts queue attributes for SQS.
// For example, StringValue "42" becomes an AWS MessageAttributeValue with
// StringValue "42".
func encodeQueueAttributes(attributes map[string]SendMessageAttribute) map[string]sqstypes.MessageAttributeValue {
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
// For example, an AWS StringValue "42" becomes MessageAttribute{StringValue:
// "42"}.
func decodeQueueAttributes(attributes map[string]sqstypes.MessageAttributeValue) map[string]messageAttribute {
	decoded := make(map[string]messageAttribute, len(attributes))
	for name, attribute := range attributes {
		decoded[name] = messageAttribute{StringValue: awscore.ToString(attribute.StringValue)}
	}

	return decoded
}

// toInt32 converts an int to int32 with bounds checking for AWS SDK inputs.
// For example, 10 becomes int32(10), while values outside int32 range are
// rejected.
func toInt32(value int, field string) (int32, error) {
	if value < -2_147_483_648 || value > 2_147_483_647 {
		return 0, fmt.Errorf("%s out of int32 range", field)
	}

	return int32(value), nil
}
