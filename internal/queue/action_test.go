package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProducerEnqueueSendsContractRequest(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	action := Action{Body: BodyTopTen, Attributes: map[string]string{"action": ActionTopTen}}

	err := (Producer{QueueURL: "queue-url", Sender: sender}).Enqueue(context.Background(), action)

	require.NoError(t, err)
	assert.Equal(t, []SendMessageRequest{BuildSendMessageRequest("queue-url", action)}, sender.requests)
}

func TestProducerEnqueueRequiresSender(t *testing.T) {
	t.Parallel()

	err := (Producer{}).Enqueue(context.Background(), Action{})

	require.Error(t, err)
	assert.EqualError(t, err, "queue sender is required")
}

func TestProducerEnqueueWrapsSenderError(t *testing.T) {
	t.Parallel()

	err := (Producer{Sender: &fakeSender{err: errors.New("boom")}}).Enqueue(context.Background(), Action{})

	require.Error(t, err)
	assert.EqualError(t, err, "send SQS message: boom")
}

func TestConsumerPollReceivesAndHandlesMessages(t *testing.T) {
	t.Parallel()

	rawMessages := []RawMessage{{ReceiptHandle: "one"}, {ReceiptHandle: "two"}}
	receiver := &fakeReceiver{response: ReceiveMessageResponse{Messages: rawMessages}}
	handled := make([]RawMessage, 0, len(rawMessages))

	err := (Consumer{QueueURL: "queue-url", Receiver: receiver}).Poll(context.Background(), func(ctx context.Context, message RawMessage) error {
		handled = append(handled, message)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, ReceiveMessageRequest{QueueURL: "queue-url", MaxNumberOfMessages: 10, WaitTimeSeconds: 20}, receiver.request)
	assert.Equal(t, rawMessages, handled)
}

func TestConsumerPollUsesConfiguredReceiveOptions(t *testing.T) {
	t.Parallel()

	receiver := &fakeReceiver{}

	err := (Consumer{QueueURL: "queue-url", Receiver: receiver, MaxNumberOfMessages: 2, WaitTimeSeconds: 5}).Poll(context.Background(), func(ctx context.Context, message RawMessage) error {
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, ReceiveMessageRequest{QueueURL: "queue-url", MaxNumberOfMessages: 2, WaitTimeSeconds: 5}, receiver.request)
}

func TestConsumerPollRequiresReceiver(t *testing.T) {
	t.Parallel()

	err := (Consumer{}).Poll(context.Background(), func(ctx context.Context, message RawMessage) error { return nil })

	require.Error(t, err)
	assert.EqualError(t, err, "queue receiver is required")
}

func TestConsumerPollRequiresHandler(t *testing.T) {
	t.Parallel()

	err := (Consumer{Receiver: &fakeReceiver{}}).Poll(context.Background(), nil)

	require.Error(t, err)
	assert.EqualError(t, err, "queue handler is required")
}

func TestConsumerPollWrapsReceiveError(t *testing.T) {
	t.Parallel()

	err := (Consumer{Receiver: &fakeReceiver{err: errors.New("boom")}}).Poll(context.Background(), func(ctx context.Context, message RawMessage) error { return nil })

	require.Error(t, err)
	assert.EqualError(t, err, "receive SQS messages: boom")
}

func TestConsumerPollReturnsHandlerError(t *testing.T) {
	t.Parallel()

	err := (Consumer{Receiver: &fakeReceiver{response: ReceiveMessageResponse{Messages: []RawMessage{{}}}}}).Poll(context.Background(), func(ctx context.Context, message RawMessage) error {
		return errors.New("boom")
	})

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestDecodeMessageSupportsStringValueCasing(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "upper case",
			raw:  `{"body":{"chatId":123},"messageAttributes":{"action":{"StringValue":"topten"}}}`,
		},
		{
			name: "lower case",
			raw:  `{"body":{"chatId":123},"messageAttributes":{"action":{"stringValue":"topten"}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var message RawMessage
			require.NoError(t, json.Unmarshal([]byte(test.raw), &message))

			action, err := DecodeMessage(message)
			require.NoError(t, err)

			assert.Equal(t, ActionTopTen, action.Name)
			assert.JSONEq(t, `{"chatId":123}`, string(action.Payload))
			assert.Equal(t, `{"chatId":123}`, action.Body)
			assert.Equal(t, "topten", action.Attributes["action"])
		})
	}
}

func TestDecodeMessagePrefersLowerCaseStringValue(t *testing.T) {
	t.Parallel()

	var message RawMessage
	require.NoError(t, json.Unmarshal([]byte(`{
		"messageAttributes": {
			"action": {"StringValue": "topten", "stringValue": "alljung"}
		}
	}`), &message))

	action, err := DecodeMessage(message)
	require.NoError(t, err)

	assert.Equal(t, ActionAllJung, action.Name)
	assert.Equal(t, ActionAllJung, action.Attributes["action"])
}

func TestDecodeEventUsesFirstContractRecord(t *testing.T) {
	t.Parallel()

	var event Event
	require.NoError(t, json.Unmarshal([]byte(`{
		"Records": [{
			"receiptHandle": "receipt",
			"messageAttributes": {
				"action": {"stringValue": "topten"},
				"chatId": {"stringValue": "123"}
			}
		}]
	}`), &event))

	action, err := DecodeEvent(event)
	require.NoError(t, err)

	assert.Equal(t, ActionTopTen, action.Name)
	assert.Equal(t, "123", action.Attributes["chatId"])
}

func TestDecodeEventRejectsMissingRecords(t *testing.T) {
	t.Parallel()

	_, err := DecodeEvent(Event{})

	require.Error(t, err)
	assert.EqualError(t, err, "missing SQS event record")
}

func TestMessageAttributeRejectsMalformedJSON(t *testing.T) {
	var attribute MessageAttribute

	err := attribute.UnmarshalJSON([]byte(`[]`))

	require.Error(t, err)
}

func TestDecodeMessagePreservesContractAttributes(t *testing.T) {
	var message RawMessage
	require.NoError(t, json.Unmarshal([]byte(`{
		"body": "sendSetOffFromWorkMessage",
		"messageAttributes": {
			"chatId": {"stringValue": "-123"},
			"chatTitle": {"stringValue": "chatTitle"},
			"userId": {"stringValue": "123"},
			"offTime": {"stringValue": "0000"},
			"workday": {"stringValue": "MON"},
			"action": {"stringValue": "setOffFromWorkTimeUTC"}
		}
	}`), &message))

	action, err := DecodeMessage(message)
	require.NoError(t, err)

	assert.Equal(t, ActionSetOffWorkTime, action.Name)
	assert.Equal(t, "sendSetOffFromWorkMessage", action.Body)
	assert.Equal(t, "-123", action.Attributes["chatId"])
	assert.Equal(t, "chatTitle", action.Attributes["chatTitle"])
	assert.Equal(t, "123", action.Attributes["userId"])
	assert.Equal(t, "0000", action.Attributes["offTime"])
	assert.Equal(t, "MON", action.Attributes["workday"])
}

func TestDecodeMessageRejectsMissingAction(t *testing.T) {
	action, err := DecodeMessage(RawMessage{})

	require.Error(t, err)
	assert.Empty(t, action.Name)
}

func TestActionNamesRemainStable(t *testing.T) {
	assert.Equal(t, "junghelp", ActionJungHelp)
	assert.Equal(t, "topten", ActionTopTen)
	assert.Equal(t, "topdiver", ActionTopDiver)
	assert.Equal(t, "alljung", ActionAllJung)
	assert.Equal(t, "enableAllJung", ActionEnableAllJung)
	assert.Equal(t, "disableAllJung", ActionDisableAllJung)
	assert.Equal(t, "setOffFromWorkTimeUTC", ActionSetOffWorkTime)
	assert.Equal(t, "offFromWork", ActionOffFromWork)
	assert.Equal(t, "onOffFromWork", ActionOnOffFromWork)
}

func TestContractBodiesRemainStable(t *testing.T) {
	assert.Equal(t, "sendJungHelpMessage", BodyJungHelp)
	assert.Equal(t, "sendTopTenMessage", BodyTopTen)
	assert.Equal(t, "sendTopDiverMessage", BodyTopDiver)
	assert.Equal(t, "sendAllJungMessage", BodyAllJung)
	assert.Equal(t, "sendEnableAllJungMessage", BodyEnableAllJung)
	assert.Equal(t, "sendDisableAllJungMessage", BodyDisableAllJung)
	assert.Equal(t, "sendSetOffFromWorkTimeUTC", BodySetOffWorkTime)
	assert.Equal(t, "sendOffFromWorkMessage", BodyOffFromWork)
	assert.Equal(t, "sendOnOffFromWork", BodyOnOffFromWork)
}

func TestBuildSendMessageRequest(t *testing.T) {
	action := Action{
		Body: BodySetOffWorkTime,
		Attributes: map[string]string{
			"action":    ActionSetOffWorkTime,
			"chatId":    "-123",
			"userId":    "456",
			"chatTitle": "Group",
		},
	}

	request := BuildSendMessageRequest("queue-url", action)

	assert.Equal(t, "queue-url", request.QueueURL)
	assert.Equal(t, BodySetOffWorkTime, request.MessageBody)
	assert.Equal(t, SendMessageAttribute{DataType: "String", StringValue: ActionSetOffWorkTime}, request.MessageAttributes["action"])
	assert.Equal(t, SendMessageAttribute{DataType: "Number", StringValue: "-123"}, request.MessageAttributes["chatId"])
	assert.Equal(t, SendMessageAttribute{DataType: "Number", StringValue: "456"}, request.MessageAttributes["userId"])
	assert.Equal(t, SendMessageAttribute{DataType: "String", StringValue: "Group"}, request.MessageAttributes["chatTitle"])
}

func TestBuildDeleteMessageRequest(t *testing.T) {
	request := BuildDeleteMessageRequest("queue-url", RawMessage{ReceiptHandle: "receipt"})

	assert.Equal(t, DeleteMessageRequest{QueueURL: "queue-url", ReceiptHandle: "receipt"}, request)
}

type fakeSender struct {
	requests []SendMessageRequest
	err      error
}

func (sender *fakeSender) SendMessage(ctx context.Context, request SendMessageRequest) error {
	sender.requests = append(sender.requests, request)
	return sender.err
}

type fakeReceiver struct {
	request  ReceiveMessageRequest
	response ReceiveMessageResponse
	err      error
}

func (receiver *fakeReceiver) ReceiveMessage(ctx context.Context, request ReceiveMessageRequest) (ReceiveMessageResponse, error) {
	receiver.request = request
	return receiver.response, receiver.err
}
