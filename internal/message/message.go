// Package message owns persisted Telegram message models and contract helpers.
package message

import (
	"context"
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

// UpdateExpression describes the contract DynamoDB update request shape without
// depending on the AWS SDK.
type UpdateExpression struct {
	TableName                 string
	Key                       map[string]any
	UpdateExpression          string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]any
}

type QueryRequest struct {
	TableName  string
	ChatID     int64
	Since      time.Time
	Until      time.Time
	Descending bool
}

type RepositoryClient interface {
	Update(ctx context.Context, request UpdateExpression) error
	QueryByChat(ctx context.Context, request QueryRequest) ([]Message, error)
}

type Repository struct {
	TableName string
	Client    RepositoryClient
	Now       func() time.Time
}

// Save stores a Telegram message row.
func (repository Repository) Save(ctx context.Context, message Message) error {
	if repository.Client == nil {
		return fmt.Errorf("message repository client is required")
	}
	if message.DateCreated.IsZero() {
		message.DateCreated = repository.now()
	}
	if message.TTL == 0 {
		message.TTL = TTL(repository.now(), DefaultTTL)
	}
	err := repository.Client.Update(ctx, BuildSaveUpdate(repository.TableName, message))
	if err != nil {
		return fmt.Errorf("save message: %w", err)
	}

	return nil
}

// QueryByChat loads messages for a chat in descending order.
func (repository Repository) QueryByChat(ctx context.Context, chatID int64, since time.Time, until time.Time) ([]Message, error) {
	if repository.Client == nil {
		return nil, fmt.Errorf("message repository client is required")
	}
	rows, err := repository.Client.QueryByChat(ctx, QueryRequest{
		TableName:  repository.TableName,
		ChatID:     chatID,
		Since:      since,
		Until:      until,
		Descending: true,
	})
	if err != nil {
		return nil, fmt.Errorf("query messages by chat: %w", err)
	}

	return rows, nil
}

// now returns the repository clock value.
func (repository Repository) now() time.Time {
	if repository.Now == nil {
		return time.Now()
	}

	return repository.Now()
}

// FromTelegram converts a Telegram message into the stored message model.
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
func FormatDateCreated(timestamp time.Time) string {
	return timestamp.In(storageLocation).Format(time.RFC3339)
}

// ParseDateCreated parses existing DynamoDB dateCreated strings.
func ParseDateCreated(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse dateCreated: %w", err)
	}

	return parsed, nil
}

// TTL returns the Unix timestamp used by the contract ttl attribute.
func TTL(now time.Time, retention time.Duration) int64 {
	return now.Add(retention).Unix()
}

// BuildSaveUpdate builds the contract DynamoDB update shape for a message row.
func BuildSaveUpdate(tableName string, message Message) UpdateExpression {
	attributeNames := map[string]string{
		"#ttl": "ttl",
	}
	attributeValues := map[string]any{
		":ttl": message.TTL,
	}
	assignments := []string{"#ttl = :ttl"}

	assignments = addStringAttribute(assignments, attributeNames, attributeValues, "chatTitle", message.ChatTitle)
	assignments = addIntAttribute(assignments, attributeNames, attributeValues, "userId", message.UserID)
	assignments = addStringAttribute(assignments, attributeNames, attributeValues, "username", message.Username)
	assignments = addStringAttribute(assignments, attributeNames, attributeValues, "firstName", message.FirstName)
	assignments = addStringAttribute(assignments, attributeNames, attributeValues, "lastName", message.LastName)

	return UpdateExpression{
		TableName: tableName,
		Key: map[string]any{
			"chatId":      message.ChatID,
			"dateCreated": FormatDateCreated(message.DateCreated),
		},
		UpdateExpression:          "SET " + joinAssignments(assignments),
		ExpressionAttributeNames:  attributeNames,
		ExpressionAttributeValues: attributeValues,
	}
}

// addStringAttribute adds a non-empty string attribute to an update.
func addStringAttribute(
	assignments []string,
	names map[string]string,
	values map[string]any,
	attribute string,
	value string,
) []string {
	if value == "" {
		return assignments
	}

	placeholder := "#" + attribute
	valuePlaceholder := ":" + attribute
	names[placeholder] = attribute
	values[valuePlaceholder] = value
	return append(assignments, placeholder+" = "+valuePlaceholder)
}

// addIntAttribute adds a non-zero integer attribute to an update.
func addIntAttribute(
	assignments []string,
	names map[string]string,
	values map[string]any,
	attribute string,
	value int64,
) []string {
	if value == 0 {
		return assignments
	}

	placeholder := "#" + attribute
	valuePlaceholder := ":" + attribute
	names[placeholder] = attribute
	values[valuePlaceholder] = value
	return append(assignments, placeholder+" = "+valuePlaceholder)
}

// joinAssignments joins update assignments with commas.
func joinAssignments(assignments []string) string {
	var result strings.Builder
	result.WriteString(assignments[0])
	for _, assignment := range assignments[1:] {
		result.WriteString(", " + assignment)
	}

	return result.String()
}
