package queue

import (
	"context"
	"fmt"
	"strconv"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=sqs.go -destination=../mock/queue_mock.go -package=mock -mock_names queueRequester=MockQueueRequester,dependencyObserver=MockQueueDependencyObserver"

// queueRequester is the SQS SDK surface used by the queue adapter.
type queueRequester interface {
	DeleteMessage(ctx context.Context, params *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error)
	DeleteMessageBatch(ctx context.Context, params *awssqs.DeleteMessageBatchInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageBatchOutput, error)
	ReceiveMessage(ctx context.Context, params *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error)
	SendMessage(ctx context.Context, params *awssqs.SendMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error)
	SendMessageBatch(ctx context.Context, params *awssqs.SendMessageBatchInput, optFns ...func(*awssqs.Options)) (*awssqs.SendMessageBatchOutput, error)
}

// dependencyObserver records a completed SQS request.
type dependencyObserver interface {
	ObserveDependency(dependency string, operation string, duration time.Duration, err error)
}

// sqsClient adapts the AWS SQS SDK to the queue package contracts.
type sqsClient struct {
	queue   queueRequester
	metrics dependencyObserver
}

// NewClient builds an SQS-backed queue client.
func NewClient(queue queueRequester, metrics ...dependencyObserver) sqsClient {
	return sqsClient{queue: queue, metrics: firstMetrics(metrics)}
}

// Delete removes a consumed SQS message.
func (client sqsClient) Delete(ctx context.Context, request DeleteMessageRequest) error {
	if client.queue == nil {
		return fmt.Errorf("queue client is required")
	}

	started := time.Now()
	_, err := client.queue.DeleteMessage(ctx, &awssqs.DeleteMessageInput{
		QueueUrl:      awscore.String(request.QueueURL),
		ReceiptHandle: awscore.String(request.ReceiptHandle),
	})
	client.observe("delete", started, err)
	if err != nil {
		return fmt.Errorf("delete SQS message: %w", err)
	}

	return nil
}

// DeleteBatch removes up to ten consumed SQS messages.
func (client sqsClient) DeleteBatch(ctx context.Context, request DeleteMessageBatchRequest) error {
	if client.queue == nil {
		return fmt.Errorf("queue client is required")
	}

	entries := make([]sqstypes.DeleteMessageBatchRequestEntry, 0, len(request.ReceiptHandles))
	for index, receiptHandle := range request.ReceiptHandles {
		entries = append(entries, sqstypes.DeleteMessageBatchRequestEntry{
			Id:            awscore.String(strconv.Itoa(index)),
			ReceiptHandle: awscore.String(receiptHandle),
		})
	}
	started := time.Now()
	output, err := client.queue.DeleteMessageBatch(ctx, &awssqs.DeleteMessageBatchInput{
		QueueUrl: awscore.String(request.QueueURL),
		Entries:  entries,
	})
	client.observe("delete_batch", started, err)
	if err != nil {
		return fmt.Errorf("delete SQS message batch: %w", err)
	}
	if len(output.Failed) > 0 {
		return fmt.Errorf("delete SQS message batch: %d entries failed", len(output.Failed))
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

	started := time.Now()
	output, err := client.queue.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
		QueueUrl:              awscore.String(request.QueueURL),
		MaxNumberOfMessages:   maxMessages,
		MessageAttributeNames: []string{"All"},
		WaitTimeSeconds:       waitSeconds,
	})
	client.observe("receive", started, err)
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

	input := &awssqs.SendMessageInput{
		QueueUrl:          awscore.String(request.QueueURL),
		MessageBody:       awscore.String(request.MessageBody),
		MessageAttributes: encodeQueueAttributes(request.MessageAttributes),
	}
	if request.MessageGroupID != "" {
		input.MessageGroupId = awscore.String(request.MessageGroupID)
	}
	if request.MessageDeduplicationID != "" {
		input.MessageDeduplicationId = awscore.String(request.MessageDeduplicationID)
	}
	started := time.Now()
	_, err := client.queue.SendMessage(ctx, input)
	client.observe("send", started, err)
	if err != nil {
		return fmt.Errorf("send SQS message: %w", err)
	}

	return nil
}

// SendMessageBatch sends up to ten queue actions to SQS.
func (client sqsClient) SendMessageBatch(ctx context.Context, request SendMessageBatchRequest) error {
	if client.queue == nil {
		return fmt.Errorf("queue client is required")
	}

	entries := make([]sqstypes.SendMessageBatchRequestEntry, 0, len(request.Entries))
	for _, entry := range request.Entries {
		requestEntry := sqstypes.SendMessageBatchRequestEntry{
			Id:                awscore.String(entry.ID),
			MessageBody:       awscore.String(entry.MessageBody),
			MessageAttributes: encodeQueueAttributes(entry.MessageAttributes),
		}
		if entry.MessageGroupID != "" {
			requestEntry.MessageGroupId = awscore.String(entry.MessageGroupID)
		}
		if entry.MessageDeduplicationID != "" {
			requestEntry.MessageDeduplicationId = awscore.String(entry.MessageDeduplicationID)
		}
		entries = append(entries, requestEntry)
	}
	started := time.Now()
	output, err := client.queue.SendMessageBatch(ctx, &awssqs.SendMessageBatchInput{
		QueueUrl: awscore.String(request.QueueURL),
		Entries:  entries,
	})
	client.observe("send_batch", started, err)
	if err != nil {
		return fmt.Errorf("send SQS message batch: %w", err)
	}
	if len(output.Failed) > 0 {
		return fmt.Errorf("send SQS message batch: %d entries failed", len(output.Failed))
	}

	return nil
}

// firstMetrics returns the optional dependency observer from application wiring.
func firstMetrics(metrics []dependencyObserver) dependencyObserver {
	if len(metrics) == 0 {
		return nil
	}

	return metrics[0]
}

// observe records a completed SQS request when metrics are enabled.
func (client sqsClient) observe(operation string, started time.Time, err error) {
	if client.metrics != nil {
		client.metrics.ObserveDependency("sqs", operation, time.Since(started), err)
	}
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
