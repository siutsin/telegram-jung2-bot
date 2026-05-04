// Package message owns persisted Telegram message models and contract helpers.
package message

import (
	"fmt"
	"strings"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

const (
	// DefaultTTL is the contract message retention duration.
	DefaultTTL = 7 * 24 * time.Hour

	storageOffsetSeconds = 8 * 60 * 60
)

var storageLocation = time.FixedZone("UTC+8", storageOffsetSeconds)

// Message is the stored Telegram message model.
type Message struct {
	ChatID      int64
	DateCreated time.Time
	ChatTitle   string
	UserID      int64
	Username    string
	FirstName   string
	LastName    string
	TTL         int64
}

// updateExpression describes the contract DynamoDB update request shape without
// depending on the AWS SDK.
type updateExpression struct {
	TableName                 string
	Key                       map[string]any
	UpdateExpression          string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]any
}

// FromTelegram converts a Telegram message into the stored message model.
// For example, a Telegram chat ID 42 becomes Message{ChatID: 42} with
// DateCreated stored in UTC+8 format.
func FromTelegram(input telegram.Message, now time.Time) Message {
	message := Message{
		ChatID:      input.Chat.ID,
		DateCreated: now.In(storageLocation),
		ChatTitle:   input.Chat.Title,
		TTL:         TTL(now, DefaultTTL),
	}

	if input.From != nil {
		message.UserID = input.From.ID
		message.Username = input.From.UserName
		message.FirstName = input.From.FirstName
		message.LastName = input.From.LastName
	}

	return message
}

// FormatDateCreated formats the DynamoDB sort key in the contract UTC+8 format.
// For example, midnight UTC becomes "2006-01-02T08:00:00+08:00".
func FormatDateCreated(timestamp time.Time) string {
	return timestamp.In(storageLocation).Format(time.RFC3339)
}

// ParseDateCreated parses existing DynamoDB dateCreated strings.
// For example, "2006-01-02T08:00:00+08:00" becomes the same instant as a
// time.Time.
func ParseDateCreated(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse dateCreated: %w", err)
	}

	return parsed, nil
}

// TTL returns the Unix timestamp used by the contract ttl attribute.
// For example, now plus seven days becomes the Unix expiry stored in ttl.
func TTL(now time.Time, retention time.Duration) int64 {
	return now.Add(retention).Unix()
}

// BuildSaveUpdate builds the contract DynamoDB update shape for a message row.
// For example, a message with username and firstName adds only those non-empty
// fields to the SET clause.
func BuildSaveUpdate(tableName string, message Message) updateExpression {
	attributeNames := make(map[string]string)
	attributeValues := make(map[string]any)
	assignments := make([]string, 0, 6)

	attributes := []struct {
		name  string
		value any
	}{
		{name: "chatTitle", value: message.ChatTitle},
		{name: "userId", value: message.UserID},
		{name: "username", value: message.Username},
		{name: "firstName", value: message.FirstName},
		{name: "lastName", value: message.LastName},
		{name: "ttl", value: message.TTL},
	}
	for _, attribute := range attributes {
		assignments = addAttribute(assignments, attributeNames, attributeValues, attribute.name, attribute.value)
	}

	return updateExpression{
		TableName: tableName,
		Key: map[string]any{
			"chatId":      message.ChatID,
			"dateCreated": FormatDateCreated(message.DateCreated),
		},
		UpdateExpression:          "SET " + strings.Join(assignments, ", "),
		ExpressionAttributeNames:  attributeNames,
		ExpressionAttributeValues: attributeValues,
	}
}

// addAttribute adds a non-zero contract attribute to an update.
// For example, "username", "alice" appends "#username = :username".
func addAttribute(
	assignments []string,
	names map[string]string,
	values map[string]any,
	name string,
	value any,
) []string {
	if isZeroAttributeValue(value) {
		return assignments
	}

	placeholder := "#" + name
	valuePlaceholder := ":" + name
	names[placeholder] = name
	values[valuePlaceholder] = value
	return append(assignments, placeholder+" = "+valuePlaceholder)
}

func isZeroAttributeValue(value any) bool {
	switch typedValue := value.(type) {
	case string:
		return typedValue == ""
	case int64:
		return typedValue == 0
	default:
		return value == nil
	}
}
