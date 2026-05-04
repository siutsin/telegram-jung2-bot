// Package message owns persisted Telegram message models and contract helpers.
package message

import (
	"fmt"
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
