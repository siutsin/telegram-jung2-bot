// Package chat owns persisted chat settings models and contract helpers.
package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

const defaultOffTime = "1000"
const scheduleWorkdayMask = workday.Sun | workday.Mon | workday.Tue | workday.Wed | workday.Thu | workday.Fri | workday.Sat

// Settings is the persisted chat settings model.
type Settings struct {
	ChatID        int64
	ChatTitle     string
	DateCreated   time.Time
	TTL           int64
	EnableAllJung bool
	OffTime       string
	Workday       workday.Workdays
	HasOffTime    bool
	HasWorkday    bool
}

// Row is the loose DynamoDB row shape used before validation/defaulting.
type Row struct {
	ChatID        int64
	ChatTitle     string
	DateCreated   string
	TTL           int64
	EnableAllJung *bool
	OffTime       string
	Workday       *int
}

// UpdateExpression describes a contract DynamoDB update request shape without an
// AWS SDK dependency.
type UpdateExpression struct {
	TableName                 string
	Key                       map[string]any
	UpdateExpression          string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]any
}

type RepositoryClient interface {
	Get(ctx context.Context, tableName string, chatID int64) (Row, bool, error)
	Update(ctx context.Context, request UpdateExpression) error
	ListEnabled(ctx context.Context, tableName string) ([]Row, error)
}

type Repository struct {
	TableName string
	Client    RepositoryClient
}

// Get loads chat settings by chat ID.
func (repository Repository) Get(ctx context.Context, chatID int64) (Settings, error) {
	if repository.Client == nil {
		return Settings{}, fmt.Errorf("chat repository client is required")
	}
	row, ok, err := repository.Client.Get(ctx, repository.TableName, chatID)
	if err != nil {
		return Settings{}, fmt.Errorf("get chat settings: %w", err)
	}
	if !ok {
		row = Row{ChatID: chatID}
	}
	settings, err := FromRow(row)
	if err != nil {
		return Settings{}, fmt.Errorf("parse chat settings: %w", err)
	}

	return settings, nil
}

// Save stores chat settings.
func (repository Repository) Save(ctx context.Context, settings Settings) error {
	if repository.Client == nil {
		return fmt.Errorf("chat repository client is required")
	}
	if err := repository.Client.Update(ctx, BuildMetadataUpdate(repository.TableName, settings)); err != nil {
		return fmt.Errorf("save chat settings: %w", err)
	}

	return nil
}

// ListEnabled loads chats with scheduling enabled.
func (repository Repository) ListEnabled(ctx context.Context) ([]Settings, error) {
	if repository.Client == nil {
		return nil, fmt.Errorf("chat repository client is required")
	}
	rows, err := repository.Client.ListEnabled(ctx, repository.TableName)
	if err != nil {
		return nil, fmt.Errorf("list enabled chats: %w", err)
	}
	settings := make([]Settings, 0, len(rows))
	for _, row := range rows {
		settings = append(settings, scheduleSettingsFromRow(row))
	}

	return settings, nil
}

// FromTelegram converts a Telegram chat into persisted chat settings metadata.
func FromTelegram(input telegram.Message, now time.Time) Settings {
	return Settings{
		ChatID:        input.Chat.ID,
		ChatTitle:     input.Chat.Title,
		DateCreated:   now,
		TTL:           message.TTL(now, message.DefaultTTL),
		EnableAllJung: true,
	}
}

// FromRow applies contract defaults to a stored chat row.
func FromRow(row Row) (Settings, error) {
	enableAllJung := true
	if row.EnableAllJung != nil {
		enableAllJung = *row.EnableAllJung
	}

	settings := Settings{
		ChatID:        row.ChatID,
		ChatTitle:     row.ChatTitle,
		TTL:           row.TTL,
		EnableAllJung: enableAllJung,
		OffTime:       row.OffTime,
		HasOffTime:    row.OffTime != "",
	}

	if row.DateCreated != "" {
		dateCreated, err := message.ParseDateCreated(row.DateCreated)
		if err != nil {
			return Settings{}, err
		}
		settings.DateCreated = dateCreated
	}

	if row.Workday != nil {
		parsed, err := workday.Parse(*row.Workday)
		if err != nil {
			return Settings{}, err
		}
		settings.Workday = parsed
		settings.HasWorkday = true
	}

	return settings, nil
}

// BuildMetadataUpdate builds the contract chat metadata update request.
func BuildMetadataUpdate(tableName string, settings Settings) UpdateExpression {
	return UpdateExpression{
		TableName:        tableName,
		Key:              map[string]any{"chatId": settings.ChatID},
		UpdateExpression: "SET #ct = :ct, #dc = :dc, #ttl = :ttl",
		ExpressionAttributeNames: map[string]string{
			"#ct":  "chatTitle",
			"#dc":  "dateCreated",
			"#ttl": "ttl",
		},
		ExpressionAttributeValues: map[string]any{
			":ct":  settings.ChatTitle,
			":dc":  message.FormatDateCreated(settings.DateCreated),
			":ttl": settings.TTL,
		},
	}
}

// BuildAllJungUpdate builds the contract enableAllJung update request.
func BuildAllJungUpdate(tableName string, chatID int64, enabled bool) UpdateExpression {
	return UpdateExpression{
		TableName:        tableName,
		Key:              map[string]any{"chatId": chatID},
		UpdateExpression: "SET #eaj = :eaj",
		ExpressionAttributeNames: map[string]string{
			"#eaj": "enableAllJung",
		},
		ExpressionAttributeValues: map[string]any{
			":eaj": enabled,
		},
	}
}

// BuildOffWorkUpdate builds the contract off-work setting update request.
func BuildOffWorkUpdate(tableName string, chatID int64, offTime string, workdays workday.Workdays) UpdateExpression {
	return UpdateExpression{
		TableName:        tableName,
		Key:              map[string]any{"chatId": chatID},
		UpdateExpression: "SET #ot = :ot, #wd = :wd",
		ExpressionAttributeNames: map[string]string{
			"#ot": "offTime",
			"#wd": "workday",
		},
		ExpressionAttributeValues: map[string]any{
			":ot": offTime,
			":wd": int(workdays),
		},
	}
}

// FilterDue returns chats due for a scheduled off-work report.
func FilterDue(rows []Settings, offTime string, day string) []Settings {
	due := make([]Settings, 0, len(rows))
	for _, row := range rows {
		if isDue(row, offTime, day) {
			due = append(due, row)
		}
	}

	return due
}

// isDue reports whether settings match the given schedule window.
func isDue(settings Settings, offTime string, day string) bool {
	if !settings.HasOffTime && !settings.HasWorkday {
		return offTime == defaultOffTime && workday.MatchesDay(day, workday.Workdays(workday.Mon|workday.Tue|workday.Wed|workday.Thu|workday.Fri))
	}
	if settings.OffTime != offTime || !settings.HasWorkday {
		return false
	}

	return workday.MatchesDay(day, settings.Workday)
}

// scheduleSettingsFromRow loads only the fields used by scheduled fan-out.
func scheduleSettingsFromRow(row Row) Settings {
	enableAllJung := true
	if row.EnableAllJung != nil {
		enableAllJung = *row.EnableAllJung
	}

	settings := Settings{
		ChatID:        row.ChatID,
		ChatTitle:     row.ChatTitle,
		TTL:           row.TTL,
		EnableAllJung: enableAllJung,
		OffTime:       row.OffTime,
		HasOffTime:    row.OffTime != "",
	}

	if row.Workday != nil {
		settings.Workday = workday.Workdays(*row.Workday & scheduleWorkdayMask)
		settings.HasWorkday = true
	}

	return settings
}
