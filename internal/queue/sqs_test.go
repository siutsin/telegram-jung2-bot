package queue

import (
	"context"
	"testing"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientReceiveMessageSupportsContractAttributes(t *testing.T) {
	t.Parallel()

	client := NewClient(&fakeSQSAPI{
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
	})
	response, err := client.ReceiveMessage(context.Background(), ReceiveMessageRequest{
		MaxNumberOfMessages: 10,
		QueueURL:            "queue-url",
		WaitTimeSeconds:     20,
	})

	require.NoError(t, err)
	require.Len(t, response.Messages, 1)
	assert.Equal(t, "receipt", response.Messages[0].ReceiptHandle)
	assert.Equal(t, "topten", response.Messages[0].MessageAttributes["action"].StringValue)
}

func TestClientSendMessageEncodesAttributes(t *testing.T) {
	t.Parallel()

	queueAPI := &fakeSQSAPI{}

	client := NewClient(queueAPI)
	err := client.SendMessage(context.Background(), SendMessageRequest{
		QueueURL:    "queue-url",
		MessageBody: BodyTopTen,
		MessageAttributes: map[string]SendMessageAttribute{
			"action": {DataType: "String", StringValue: ActionTopTen},
		},
	})

	require.NoError(t, err)
	require.Len(t, queueAPI.sendInputs, 1)
	assert.Equal(t, BodyTopTen, awscore.ToString(queueAPI.sendInputs[0].MessageBody))
	assert.Equal(t, ActionTopTen, awscore.ToString(queueAPI.sendInputs[0].MessageAttributes["action"].StringValue))
}

func TestClientDeleteRemovesConsumedMessage(t *testing.T) {
	t.Parallel()

	queueAPI := &fakeSQSAPI{}

	client := NewClient(queueAPI)
	err := client.Delete(context.Background(), DeleteMessageRequest{
		QueueURL:      "queue-url",
		ReceiptHandle: "receipt",
	})

	require.NoError(t, err)
	require.Len(t, queueAPI.deleteInputs, 1)
	assert.Equal(t, "receipt", awscore.ToString(queueAPI.deleteInputs[0].ReceiptHandle))
}

type fakeSQSAPI struct {
	deleteInputs  []*awssqs.DeleteMessageInput
	receiveOutput *awssqs.ReceiveMessageOutput
	sendInputs    []*awssqs.SendMessageInput
}

func (client *fakeSQSAPI) DeleteMessage(ctx context.Context, params *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
	client.deleteInputs = append(client.deleteInputs, params)
	return &awssqs.DeleteMessageOutput{}, nil
}

func (client *fakeSQSAPI) ReceiveMessage(ctx context.Context, params *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	if client.receiveOutput != nil {
		return client.receiveOutput, nil
	}
	return &awssqs.ReceiveMessageOutput{}, nil
}

func (client *fakeSQSAPI) SendMessage(ctx context.Context, params *awssqs.SendMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error) {
	client.sendInputs = append(client.sendInputs, params)
	return &awssqs.SendMessageOutput{}, nil
}
