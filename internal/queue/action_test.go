package queue

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProducerEnqueueSendsContractRequest(t *testing.T) {
	t.Parallel()

	queueSender := &fakeSender{}
	action := Action{Body: BodyTopTen, Attributes: map[string]string{"action": ActionTopTen}}

	err := (NewProducer("queue-url", queueSender)).Enqueue(context.Background(), action)

	require.NoError(t, err)
	assert.Equal(t, []SendMessageRequest{buildSendMessageRequest("queue-url", action)}, queueSender.requests)
}

func TestProducerEnqueueRequiresSender(t *testing.T) {
	t.Parallel()

	err := (producer{}).Enqueue(context.Background(), Action{})

	require.Error(t, err)
	assert.EqualError(t, err, "queue sender is required")
}

func TestProducerEnqueueReturnsSenderError(t *testing.T) {
	t.Parallel()

	err := (producer{sender: &fakeSender{err: errors.New("boom")}}).Enqueue(context.Background(), Action{})

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestConsumerPollReceivesAndHandlesMessages(t *testing.T) {
	t.Parallel()

	rawMessages := []RawMessage{{ReceiptHandle: "one"}, {ReceiptHandle: "two"}}
	queueReceiver := &fakeReceiver{response: ReceiveMessageResponse{Messages: rawMessages}}
	handled := make([]string, 0, len(rawMessages))
	var mutex sync.Mutex

	err := (NewConsumer("queue-url", queueReceiver)).Poll(context.Background(), func(ctx context.Context, message RawMessage) error {
		mutex.Lock()
		handled = append(handled, message.ReceiptHandle)
		mutex.Unlock()
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, ReceiveMessageRequest{QueueURL: "queue-url", MaxNumberOfMessages: 10, WaitTimeSeconds: 20}, queueReceiver.request)
	slices.Sort(handled)
	assert.Equal(t, []string{"one", "two"}, handled)
}

func TestConsumerPollUsesConfiguredReceiveOptions(t *testing.T) {
	t.Parallel()

	queueReceiver := &fakeReceiver{}

	err := (consumer{queueURL: "queue-url", receiver: queueReceiver, maxNumberOfMessages: 2, waitTimeSeconds: 5}).Poll(context.Background(), func(ctx context.Context, message RawMessage) error {
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, ReceiveMessageRequest{QueueURL: "queue-url", MaxNumberOfMessages: 2, WaitTimeSeconds: 5}, queueReceiver.request)
}

func TestConsumerPollErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		consumer consumer
		handler  func(context.Context, RawMessage) error
		wantErr  string
	}{
		{
			name:     "missing receiver",
			consumer: consumer{},
			handler:  func(ctx context.Context, message RawMessage) error { return nil },
			wantErr:  "queue receiver is required",
		},
		{
			name:     "missing handler",
			consumer: consumer{receiver: &fakeReceiver{}},
			wantErr:  "queue handler is required",
		},
		{
			name:     "receive error",
			consumer: consumer{receiver: &fakeReceiver{err: errors.New("boom")}},
			handler:  func(ctx context.Context, message RawMessage) error { return nil },
			wantErr:  "boom",
		},
		{
			name:     "handler error",
			consumer: consumer{receiver: &fakeReceiver{response: ReceiveMessageResponse{Messages: []RawMessage{{}}}}},
			handler: func(ctx context.Context, message RawMessage) error {
				return errors.New("boom")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.consumer.Poll(context.Background(), test.handler)

			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
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

			action := DecodeMessage(message)

			assert.Equal(t, ActionTopTen, action.Name)
			assert.Equal(t, `{"chatId":123}`, action.Body)
			assert.Equal(t, "topten", action.Attributes["action"])
		})
	}
}

func TestDecodeMessageTreatsMissingActionAsNoOp(t *testing.T) {
	t.Parallel()

	action := DecodeMessage(RawMessage{})

	assert.Equal(t, Action{}, action)
}

func TestDecodeMessagePrefersLowerCaseStringValue(t *testing.T) {
	t.Parallel()

	var message RawMessage
	require.NoError(t, json.Unmarshal([]byte(`{
		"messageAttributes": {
			"action": {"StringValue": "topten", "stringValue": "alljung"}
		}
	}`), &message))

	action := DecodeMessage(message)

	assert.Equal(t, ActionAllJung, action.Name)
	assert.Equal(t, ActionAllJung, action.Attributes["action"])
}

func TestMessageAttributeRejectsMalformedJSON(t *testing.T) {
	var attribute messageAttribute

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

	action := DecodeMessage(message)

	assert.Equal(t, ActionSetOffWorkTime, action.Name)
	assert.Equal(t, "sendSetOffFromWorkMessage", action.Body)
	assert.Equal(t, "-123", action.Attributes["chatId"])
	assert.Equal(t, "chatTitle", action.Attributes["chatTitle"])
	assert.Equal(t, "123", action.Attributes["userId"])
	assert.Equal(t, "0000", action.Attributes["offTime"])
	assert.Equal(t, "MON", action.Attributes["workday"])
}

func TestActionNamesRemainStable(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "jung help", got: ActionJungHelp, want: "junghelp"},
		{name: "top ten", got: ActionTopTen, want: "topten"},
		{name: "top diver", got: ActionTopDiver, want: "topdiver"},
		{name: "all jung", got: ActionAllJung, want: "alljung"},
		{name: "enable all jung", got: ActionEnableAllJung, want: "enableAllJung"},
		{name: "disable all jung", got: ActionDisableAllJung, want: "disableAllJung"},
		{name: "set off work time", got: ActionSetOffWorkTime, want: "setOffFromWorkTimeUTC"},
		{name: "off from work", got: ActionOffFromWork, want: "offFromWork"},
		{name: "on off from work", got: ActionOnOffFromWork, want: "onOffFromWork"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.got)
		})
	}
}

func TestContractBodiesRemainStable(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "jung help", got: BodyJungHelp, want: "sendJungHelpMessage"},
		{name: "top ten", got: BodyTopTen, want: "sendTopTenMessage"},
		{name: "top diver", got: BodyTopDiver, want: "sendTopDiverMessage"},
		{name: "all jung", got: BodyAllJung, want: "sendAllJungMessage"},
		{name: "enable all jung", got: BodyEnableAllJung, want: "sendEnableAllJungMessage"},
		{name: "disable all jung", got: BodyDisableAllJung, want: "sendDisableAllJungMessage"},
		{name: "set off work time", got: BodySetOffWorkTime, want: "sendSetOffFromWorkTimeUTC"},
		{name: "off from work", got: BodyOffFromWork, want: "sendOffFromWorkMessage"},
		{name: "on off from work", got: BodyOnOffFromWork, want: "sendOnOffFromWork"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.got)
		})
	}
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

	request := buildSendMessageRequest("queue-url", action)

	assert.Equal(t, "queue-url", request.QueueURL)
	assert.Equal(t, BodySetOffWorkTime, request.MessageBody)
	assert.Equal(t, SendMessageAttribute{DataType: "String", StringValue: ActionSetOffWorkTime}, request.MessageAttributes["action"])
	assert.Equal(t, SendMessageAttribute{DataType: "Number", StringValue: "-123"}, request.MessageAttributes["chatId"])
	assert.Equal(t, SendMessageAttribute{DataType: "Number", StringValue: "456"}, request.MessageAttributes["userId"])
	assert.Equal(t, SendMessageAttribute{DataType: "String", StringValue: "Group"}, request.MessageAttributes["chatTitle"])
}

type fakeSender struct {
	requests []SendMessageRequest
	err      error
}

func (queueSender *fakeSender) SendMessage(ctx context.Context, request SendMessageRequest) error {
	queueSender.requests = append(queueSender.requests, request)
	return queueSender.err
}

type fakeReceiver struct {
	request  ReceiveMessageRequest
	response ReceiveMessageResponse
	err      error
}

func (queueReceiver *fakeReceiver) ReceiveMessage(ctx context.Context, request ReceiveMessageRequest) (ReceiveMessageResponse, error) {
	queueReceiver.request = request
	return queueReceiver.response, queueReceiver.err
}
