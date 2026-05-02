// Package queue contains SQS action models and decoding helpers.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
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
	Payload    json.RawMessage
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

// Event is the Lambda SQS event wrapper consumed by the contract handler.
type Event struct {
	Records []RawMessage `json:"Records"`
}

type Sender interface {
	SendMessage(ctx context.Context, request SendMessageRequest) error
}

type Receiver interface {
	ReceiveMessage(ctx context.Context, request ReceiveMessageRequest) (ReceiveMessageResponse, error)
}

type Handler func(ctx context.Context, message RawMessage) error

type Producer struct {
	QueueURL string
	Sender   Sender
}

func (producer Producer) Enqueue(ctx context.Context, action Action) error {
	if producer.Sender == nil {
		return fmt.Errorf("queue sender is required")
	}
	if err := producer.Sender.SendMessage(ctx, BuildSendMessageRequest(producer.QueueURL, action)); err != nil {
		return fmt.Errorf("send SQS message: %w", err)
	}

	return nil
}

type Consumer struct {
	QueueURL            string
	Receiver            Receiver
	MaxNumberOfMessages int
	WaitTimeSeconds     int
}

func (consumer Consumer) Poll(ctx context.Context, handler Handler) error {
	if consumer.Receiver == nil {
		return fmt.Errorf("queue receiver is required")
	}
	if handler == nil {
		return fmt.Errorf("queue handler is required")
	}
	response, err := consumer.Receiver.ReceiveMessage(ctx, ReceiveMessageRequest{
		QueueURL:            consumer.QueueURL,
		MaxNumberOfMessages: maxNumberOfMessages(consumer.MaxNumberOfMessages),
		WaitTimeSeconds:     waitTimeSeconds(consumer.WaitTimeSeconds),
	})
	if err != nil {
		return fmt.Errorf("receive SQS messages: %w", err)
	}
	for _, message := range response.Messages {
		if err := handler(ctx, message); err != nil {
			return err
		}
	}

	return nil
}

// MessageAttribute is the action attribute shape used by SQS events.
type MessageAttribute struct {
	StringValue string `json:"StringValue"`
	stringValue string
}

// UnmarshalJSON supports both contract StringValue and lower-case stringValue
// casings.
func (attribute *MessageAttribute) UnmarshalJSON(data []byte) error {
	var raw struct {
		StringValue string `json:"StringValue"`
		LowerValue  string `json:"stringValue"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode message attribute: %w", err)
	}

	attribute.StringValue = raw.StringValue
	attribute.stringValue = raw.LowerValue
	return nil
}

func (attribute MessageAttribute) value() string {
	if attribute.stringValue != "" {
		return attribute.stringValue
	}

	return attribute.StringValue
}

// RawMessage is the subset of an SQS event message needed for action dispatch.
type RawMessage struct {
	Body              json.RawMessage             `json:"body"`
	ReceiptHandle     string                      `json:"receiptHandle"`
	MessageAttributes map[string]MessageAttribute `json:"messageAttributes"`
}

// DecodeMessage converts a raw SQS event message into an action.
func DecodeMessage(message RawMessage) (Action, error) {
	attribute, ok := message.MessageAttributes["action"]
	if !ok || attribute.value() == "" {
		return Action{}, fmt.Errorf("missing action message attribute")
	}

	attributes := make(map[string]string, len(message.MessageAttributes))
	for name, attribute := range message.MessageAttributes {
		attributes[name] = attribute.value()
	}

	return Action{
		Name:       attribute.value(),
		Body:       messageBodyText(message.Body),
		Payload:    message.Body,
		Attributes: attributes,
	}, nil
}

// DecodeEvent converts the first record from a contract Lambda SQS event into an
// action, matching contract SQS.onEvent behaviour.
func DecodeEvent(event Event) (Action, error) {
	if len(event.Records) == 0 {
		return Action{}, fmt.Errorf("missing SQS event record")
	}

	return DecodeMessage(event.Records[0])
}

func messageBodyText(body json.RawMessage) string {
	var value string
	if err := json.Unmarshal(body, &value); err == nil {
		return value
	}

	return string(body)
}

// BuildSendMessageRequest converts an action into the contract SQS request shape.
func BuildSendMessageRequest(queueURL string, action Action) SendMessageRequest {
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

// BuildDeleteMessageRequest converts a consumed raw message into delete params.
func BuildDeleteMessageRequest(queueURL string, message RawMessage) DeleteMessageRequest {
	return DeleteMessageRequest{
		QueueURL:      queueURL,
		ReceiptHandle: message.ReceiptHandle,
	}
}

func maxNumberOfMessages(value int) int {
	if value > 0 {
		return value
	}

	return 10
}

func waitTimeSeconds(value int) int {
	if value > 0 {
		return value
	}

	return 20
}
