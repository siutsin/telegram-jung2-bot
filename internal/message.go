package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultTTL is the contract message retention duration.
	DefaultTTL = 7 * 24 * time.Hour

	storageOffsetSeconds = 8 * 60 * 60
)

var storageLocation = time.FixedZone("UTC+8", storageOffsetSeconds)

// StoredMessage is the persisted Telegram message model.
type StoredMessage struct {
	ChatID      int64
	DateCreated time.Time
	MessageID   int64
	ChatTitle   string
	UserID      int64
	Username    string
	FirstName   string
	LastName    string
	TTL         int64
}

// StoredMessageFromTelegram converts a Telegram message into a stored message.
// For example, a Telegram chat ID 42 becomes StoredMessage{ChatID: 42} with
// DateCreated stored in UTC+8 format.
func StoredMessageFromTelegram(input TelegramMessage, now time.Time) StoredMessage {
	dateCreated := now
	if input.Date > 0 {
		dateCreated = time.Unix(input.Date, 0)
	}
	message := StoredMessage{
		ChatID:      input.Chat.ID,
		DateCreated: dateCreated.In(storageLocation),
		MessageID:   input.MessageID,
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

// FormatMessageDateCreated formats an idempotent message sort key.
// For example, 2026-05-02T20:00:00+08:00 and message ID 42 become
// "2026-05-02T20:00:00+08:00#42".
func FormatMessageDateCreated(timestamp time.Time, messageID int64) string {
	if messageID == 0 {
		return FormatDateCreated(timestamp)
	}

	return FormatDateCreated(timestamp) + "#" + strconv.FormatInt(messageID, 10)
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
	dateCreated, _, _ := strings.Cut(raw, "#")
	parsed, err := time.Parse(time.RFC3339, dateCreated)
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
