// Package queue contains SQS action models and decoding helpers.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Stable action names used in SQS messages.
const (
	ActionJungHelp       = "junghelp"
	ActionTopTen         = "topten"
	ActionTopDiver       = "topdiver"
	ActionAllJung        = "alljung"
	ActionEnableAllJung  = "enableAllJung"
	ActionDisableAllJung = "disableAllJung"
	ActionSetOffWorkTime = "setOffFromWorkTimeUTC"
	ActionOffFromWork    = "offFromWork"
	ActionOnOffFromWork  = "onOffFromWork"
)

// Contract SQS message bodies.
const (
	BodyJungHelp       = "sendJungHelpMessage"
	BodyTopTen         = "sendTopTenMessage"
	BodyTopDiver       = "sendTopDiverMessage"
	BodyAllJung        = "sendAllJungMessage"
	BodyEnableAllJung  = "sendEnableAllJungMessage"
	BodyDisableAllJung = "sendDisableAllJungMessage"
	BodySetOffWorkTime = "sendSetOffFromWorkTimeUTC"
	BodyOffFromWork    = "sendOffFromWorkMessage"
	BodyOnOffFromWork  = "sendOnOffFromWork"
)

// Action is the service's typed representation of a queued action.
type Action struct {
	Name       string
	Body       string
	Attributes map[string]string
}

// SendMessageRequest is the SDK-free contract SQS sendMessage request shape.
type SendMessageRequest struct {
	QueueURL          string
	MessageBody       string
	MessageAttributes map[string]SendMessageAttribute
}

// SendMessageAttribute is the SDK-free SQS message attribute value shape.
type SendMessageAttribute struct {
	DataType    string
	StringValue string
}

// DeleteMessageRequest is the SDK-free contract SQS deleteMessage request shape.
type DeleteMessageRequest struct {
	QueueURL      string
	ReceiptHandle string
}

// ReceiveMessageRequest is the SDK-free SQS receiveMessage request shape.
type ReceiveMessageRequest struct {
	QueueURL            string
	MaxNumberOfMessages int
	WaitTimeSeconds     int
}

type ReceiveMessageResponse struct {
	Messages []RawMessage
}

type messageSender interface {
	SendMessage(ctx context.Context, request SendMessageRequest) error
}

type messageReceiver interface {
	ReceiveMessage(ctx context.Context, request ReceiveMessageRequest) (ReceiveMessageResponse, error)
}

type producer struct {
	queueURL string
	sender   messageSender
}

// NewProducer builds a queue producer.
func NewProducer(queueURL string, queueSender messageSender) producer {
	return producer{queueURL: queueURL, sender: queueSender}
}

// Enqueue sends an action to the queue.
func (p producer) Enqueue(ctx context.Context, action Action) error {
	if p.sender == nil {
		return fmt.Errorf("queue sender is required")
	}
	err := p.sender.SendMessage(ctx, buildSendMessageRequest(p.queueURL, action))
	if err != nil {
		return err
	}

	return nil
}

type consumer struct {
	queueURL            string
	receiver            messageReceiver
	maxNumberOfMessages int
	waitTimeSeconds     int
}

// NewConsumer builds a queue consumer with default receive options.
func NewConsumer(queueURL string, queueReceiver messageReceiver) consumer {
	return consumer{queueURL: queueURL, receiver: queueReceiver}
}

// Poll receives queue messages and dispatches them to handler.
func (c consumer) Poll(ctx context.Context, handler func(context.Context, RawMessage) error) error {
	if c.receiver == nil {
		return fmt.Errorf("queue receiver is required")
	}
	if handler == nil {
		return fmt.Errorf("queue handler is required")
	}
	response, err := c.receiver.ReceiveMessage(ctx, ReceiveMessageRequest{
		QueueURL:            c.queueURL,
		MaxNumberOfMessages: maxNumberOfMessages(c.maxNumberOfMessages),
		WaitTimeSeconds:     waitTimeSeconds(c.waitTimeSeconds),
	})
	if err != nil {
		return err
	}
	var (
		firstErr error
		mutex    sync.Mutex
		waiter   sync.WaitGroup
	)
	for _, message := range response.Messages {
		waiter.Go(func() {
			handlerErr := handler(ctx, message)
			if handlerErr != nil {
				mutex.Lock()
				if firstErr == nil {
					firstErr = handlerErr
				}
				mutex.Unlock()
			}
		})
	}
	waiter.Wait()
	if firstErr != nil {
		return firstErr
	}

	return nil
}

type messageAttribute struct {
	StringValue string `json:"StringValue"`
	stringValue string
}

// UnmarshalJSON supports both contract StringValue and lower-case stringValue
// casings. For example, {"stringValue":"42"} and {"StringValue":"42"} both
// produce the same MessageAttribute value.
func (attribute *messageAttribute) UnmarshalJSON(data []byte) error {
	var raw struct {
		StringValue string `json:"StringValue"`
		LowerValue  string `json:"stringValue"`
	}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("decode message attribute: %w", err)
	}

	attribute.StringValue = raw.StringValue
	attribute.stringValue = raw.LowerValue
	return nil
}

// value returns the message attribute value regardless of casing.
// For example, a lower-case stringValue takes priority over StringValue.
func (attribute messageAttribute) value() string {
	if attribute.stringValue != "" {
		return attribute.stringValue
	}

	return attribute.StringValue
}

// RawMessage is the subset of an SQS event message needed for action dispatch.
type RawMessage struct {
	Body              json.RawMessage             `json:"body"`
	ReceiptHandle     string                      `json:"receiptHandle"`
	MessageAttributes map[string]messageAttribute `json:"messageAttributes"`
}

// DecodeMessage converts a raw SQS event message into an action.
// For example, a raw action attribute "topTen" becomes Action{Name: "topTen"}.
func DecodeMessage(message RawMessage) (Action, error) {
	attribute, ok := message.MessageAttributes["action"]
	if !ok || attribute.value() == "" {
		return Action{}, nil
	}

	attributes := make(map[string]string, len(message.MessageAttributes))
	for name, attributeValue := range message.MessageAttributes {
		attributes[name] = attributeValue.value()
	}

	return Action{
		Name:       attribute.value(),
		Body:       messageBodyText(message.Body),
		Attributes: attributes,
	}, nil
}

// messageBodyText returns the raw body as a plain string.
// For example, JSON body "\"hello\"" becomes "hello".
func messageBodyText(body json.RawMessage) string {
	var value string
	err := json.Unmarshal(body, &value)
	if err == nil {
		return value
	}

	return string(body)
}

// buildSendMessageRequest converts an action into the contract SQS request shape.
// For example, chatId "42" becomes a Number attribute, while action stays a
// String attribute.
func buildSendMessageRequest(queueURL string, action Action) SendMessageRequest {
	attributes := make(map[string]SendMessageAttribute, len(action.Attributes))
	for name, value := range action.Attributes {
		dataType := "String"
		if name == "chatId" || name == "userId" {
			dataType = "Number"
		}
		attributes[name] = SendMessageAttribute{
			DataType:    dataType,
			StringValue: value,
		}
	}

	return SendMessageRequest{
		QueueURL:          queueURL,
		MessageBody:       action.Body,
		MessageAttributes: attributes,
	}
}

// maxNumberOfMessages returns the receive batch size.
// For example, 0 falls back to 10.
func maxNumberOfMessages(value int) int {
	if value > 0 {
		return value
	}

	return 10
}

// waitTimeSeconds returns the long-poll duration.
// For example, 0 falls back to 20.
func waitTimeSeconds(value int) int {
	if value > 0 {
		return value
	}

	return 20
}
