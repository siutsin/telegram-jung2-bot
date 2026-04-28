// Package queue contains SQS action models and decoding helpers.
package queue

import (
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

// Action is the service's typed representation of a queued action.
type Action struct {
	Name    string
	Payload json.RawMessage
}

// MessageAttribute is the action attribute shape used by SQS events.
type MessageAttribute struct {
	StringValue string `json:"StringValue"`
	stringValue string
}

// UnmarshalJSON supports both legacy StringValue and lower-case stringValue
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
	if attribute.StringValue != "" {
		return attribute.StringValue
	}

	return attribute.stringValue
}

// RawMessage is the subset of an SQS event message needed for action dispatch.
type RawMessage struct {
	Body              json.RawMessage             `json:"body"`
	MessageAttributes map[string]MessageAttribute `json:"messageAttributes"`
}

// DecodeMessage converts a raw SQS event message into an action.
func DecodeMessage(message RawMessage) (Action, error) {
	attribute, ok := message.MessageAttributes["action"]
	if !ok || attribute.value() == "" {
		return Action{}, fmt.Errorf("missing action message attribute")
	}

	return Action{
		Name:    attribute.value(),
		Payload: message.Body,
	}, nil
}
