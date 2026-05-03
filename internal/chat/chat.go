// Package chat owns persisted chat setting models and contract helpers.
package chat

import (
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

const defaultOffTime = "1000"
const scheduleWorkdayMask = workday.Sun | workday.Mon | workday.Tue | workday.Wed | workday.Thu | workday.Fri | workday.Sat
const defaultDueWorkdays = workday.Workdays(workday.Mon | workday.Tue | workday.Wed | workday.Thu | workday.Fri)

const keyChatID = "chatId"

// ChatSetting is the persisted chat-level setting record.
type ChatSetting struct {
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

// FromTelegram converts a Telegram chat into persisted chat settings metadata.
// For example, a group chat with ID 42 becomes ChatSetting{ChatID: 42,
// EnableAllJung: true}.
func FromTelegram(input telegram.Message, now time.Time) ChatSetting {
	return ChatSetting{
		ChatID:        input.Chat.ID,
		ChatTitle:     input.Chat.Title,
		DateCreated:   now,
		TTL:           message.TTL(now, message.DefaultTTL),
		EnableAllJung: true,
	}
}

// ParseRow applies contract defaults and strict parsing to a stored chat row.
// For example, a row without enableAllJung becomes ChatSetting{EnableAllJung: true}.
func ParseRow(row Row) (ChatSetting, error) {
	settings := baseSettings(row)

	if row.DateCreated != "" {
		dateCreated, err := message.ParseDateCreated(row.DateCreated)
		if err != nil {
			return ChatSetting{}, err
		}
		settings.DateCreated = dateCreated
	}

	if row.Workday != nil {
		parsed, err := workday.Parse(*row.Workday)
		if err != nil {
			return ChatSetting{}, err
		}
		settings.Workday = parsed
		settings.HasWorkday = true
	}

	return settings, nil
}

// BuildMetadataUpdate builds the contract chat metadata update request.
// For example, it turns ChatSetting{ChatID: 42, ChatTitle: "Ops"} into an update
// keyed by chatId 42 with chatTitle, dateCreated, and ttl assignments.
func BuildMetadataUpdate(tableName string, settings ChatSetting) UpdateExpression {
	return UpdateExpression{
		TableName:        tableName,
		Key:              map[string]any{keyChatID: settings.ChatID},
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
// For example, chatID 42 and enabled false becomes "SET #eaj = :eaj" with
// :eaj=false.
func BuildAllJungUpdate(tableName string, chatID int64, enabled bool) UpdateExpression {
	return UpdateExpression{
		TableName:        tableName,
		Key:              map[string]any{keyChatID: chatID},
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
// For example, chatID 42, offTime "1830", and MON|TUE becomes an update with
// offTime "1830" and workday 6.
func BuildOffWorkUpdate(tableName string, chatID int64, offTime string, workdays workday.Workdays) UpdateExpression {
	return UpdateExpression{
		TableName:        tableName,
		Key:              map[string]any{keyChatID: chatID},
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
// For example, filtering by offTime "1830" and day "MON" keeps only chats due
// at 18:30 on Monday.
func FilterDue(rows []ChatSetting, offTime string, day string) []ChatSetting {
	due := make([]ChatSetting, 0, len(rows))
	for _, row := range rows {
		if isDue(row, offTime, day) {
			due = append(due, row)
		}
	}

	return due
}

// isDue reports whether settings match the given schedule window.
// For example, settings with OffTime "1830" and Workday MON match
// offTime="1830", day="MON".
func isDue(settings ChatSetting, offTime string, day string) bool {
	if !settings.HasOffTime && !settings.HasWorkday {
		return offTime == defaultOffTime && workday.MatchesDay(day, defaultDueWorkdays)
	}
	if settings.OffTime != offTime || !settings.HasWorkday {
		return false
	}

	return workday.MatchesDay(day, settings.Workday)
}

// FromScheduleRow loads only the fields used by scheduled fan-out.
// For example, it masks row.Workday down to the stored weekday bits before
// assigning ChatSetting.Workday.
func FromScheduleRow(row Row) ChatSetting {
	settings := baseSettings(row)
	if row.Workday != nil {
		settings.Workday = workday.Workdays(*row.Workday & scheduleWorkdayMask)
		settings.HasWorkday = true
	}

	return settings
}

// baseSettings applies the shared stored chat defaults before any specialised
// row parsing.
// For example, a row without enableAllJung becomes ChatSetting{EnableAllJung: true}.
func baseSettings(row Row) ChatSetting {
	enableAllJung := true
	if row.EnableAllJung != nil {
		enableAllJung = *row.EnableAllJung
	}

	settings := ChatSetting{
		ChatID:        row.ChatID,
		ChatTitle:     row.ChatTitle,
		TTL:           row.TTL,
		EnableAllJung: enableAllJung,
		OffTime:       row.OffTime,
		HasOffTime:    row.OffTime != "",
	}

	return settings
}
