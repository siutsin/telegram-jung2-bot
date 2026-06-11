// Package schedule owns off-work scheduling and admin setting domain behaviour.
package schedule

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/command"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

// SetOffInput carries queue attributes for setOffFromWorkTimeUTC actions.
type SetOffInput struct {
	ChatID    int64
	ChatTitle string
	UserID    int64
	OffTime   string
	Workday   string
}

type window struct {
	OffTime string
	Weekday string
}

type SettingChange struct {
	Allowed bool
	Reply   string
	Update  chat.UpdateExpression
}

type chatListStore interface {
	ListEnabled(ctx context.Context, tableName string) ([]chat.ChatSetting, error)
}

type actionEnqueuer interface {
	Enqueue(ctx context.Context, action queue.Action) error
}

type chatScheduler interface {
	Sync(ctx context.Context, settings chat.ChatSetting) error
}

type scheduleService struct {
	chatTable string
	chats     chatListStore
	enqueuer  actionEnqueuer
	scheduler chatScheduler
}

// syncChat syncs one chat's schedule state.
func (schedule scheduleService) syncChat(ctx context.Context, settings chat.ChatSetting) error {
	if schedule.scheduler == nil {
		return fmt.Errorf("scheduler is required")
	}
	err := schedule.scheduler.Sync(ctx, settings)
	if err != nil {
		return fmt.Errorf("sync chat schedule: %w", err)
	}

	return nil
}

// handleDueReport enqueues reports due at timestamp.
func (schedule scheduleService) handleDueReport(ctx context.Context, timestamp time.Time) error {
	if schedule.chats == nil {
		return fmt.Errorf("chat repository is required")
	}
	if schedule.enqueuer == nil {
		return fmt.Errorf("enqueuer is required")
	}
	rows, err := schedule.chats.ListEnabled(ctx, schedule.chatTable)
	if err != nil {
		return fmt.Errorf("list due chats: %w", err)
	}
	for _, chatID := range DueChatIDs(rows, timestamp) {
		err = schedule.enqueuer.Enqueue(ctx, BuildOffFromWorkAction(chatID))
		if err != nil {
			return fmt.Errorf("enqueue due off-work report: %w", err)
		}
	}

	return nil
}

// WindowFromTime converts a timestamp into a contract schedule window.
// For example, 2025-01-06 18:30 UTC becomes OffTime "1830" and Weekday "MON".
func WindowFromTime(timestamp time.Time) window {
	timestamp = timestamp.UTC()

	return window{
		OffTime: timestamp.Format("1504"),
		Weekday: weekdayToken(timestamp.Weekday()),
	}
}

// DueChatIDs returns chat IDs due for a scheduled report.
// For example, chats due at Monday 18:30 become just their chat ID list.
func DueChatIDs(rows []chat.ChatSetting, timestamp time.Time) []int64 {
	scheduleWindow := WindowFromTime(timestamp)
	due := chat.FilterDue(rows, scheduleWindow.OffTime, scheduleWindow.Weekday)
	chatIDs := make([]int64, 0, len(due))
	for _, settings := range due {
		chatIDs = append(chatIDs, settings.ChatID)
	}

	return chatIDs
}

// ParseScheduledTime parses scheduler timeString values from HTTP or queue payloads.
// For example, "2025-01-06T18:30:00Z" and "2025-01-06T18:30:00.000Z" both parse.
func ParseScheduledTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("timeString is required")
	}

	timestamp, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse scheduled time: %w", err)
	}

	return timestamp, nil
}

// BuildOnOffFromWorkAction builds the scheduler fan-out action.
// For example, "2025-01-06T18:30:00Z" becomes an action with timeString set to
// that value.
func BuildOnOffFromWorkAction(timeString string) queue.Action {
	return queue.Action{
		Name: queue.ActionOnOffFromWork,
		Body: queue.BodyOnOffFromWork,
		Attributes: map[string]string{
			"action":     queue.ActionOnOffFromWork,
			"timeString": timeString,
		},
	}
}

// BuildOffFromWorkAction builds an off-work report action.
// For example, chatID 42 becomes an action with Attributes["chatId"] == "42".
func BuildOffFromWorkAction(chatID int64) queue.Action {
	return queue.Action{
		Name: queue.ActionOffFromWork,
		Body: queue.BodyOffFromWork,
		Attributes: map[string]string{
			"action": queue.ActionOffFromWork,
			"chatId": strconv.FormatInt(chatID, 10),
		},
	}
}

// EnableAllJung builds the enable-all-jung setting change.
// For example, an admin request for chat 42 becomes an enableAllJung update and
// reply text.
func EnableAllJung(tableName string, chatID int64, chatTitle string, isAdmin bool) SettingChange {
	if !isAdmin {
		return SettingChange{}
	}

	return SettingChange{
		Allowed: true,
		Reply:   enabledMessage(chatTitle),
		Update:  chat.BuildAllJungUpdate(tableName, chatID, true),
	}
}

// DisableAllJung builds the disable-all-jung setting change.
// For example, an admin request for chat 42 becomes a disableAllJung update and
// reply text.
func DisableAllJung(tableName string, chatID int64, chatTitle string, isAdmin bool) SettingChange {
	if !isAdmin {
		return SettingChange{}
	}

	return SettingChange{
		Allowed: true,
		Reply:   disabledMessage(chatTitle),
		Update:  chat.BuildAllJungUpdate(tableName, chatID, false),
	}
}

// SetOffFromWorkTimeUTC validates and builds the off-work update.
// For example, offTime "1830" and workdayList "MON,TUE" becomes a settings
// update with workday mask 6.
func SetOffFromWorkTimeUTC(
	tableName string,
	chatID int64,
	chatTitle string,
	isAdmin bool,
	offTime string,
	workdayList string,
) (SettingChange, error) {
	if !isAdmin {
		return SettingChange{}, nil
	}

	err := command.ValidateOffTime(offTime)
	if err != nil {
		return SettingChange{}, err
	}

	workdays, err := workday.ParseList(workdayList)
	if err != nil {
		return SettingChange{}, err
	}

	return SettingChange{
		Allowed: true,
		Reply:   setOffMessage(chatTitle, offTime, workdayList),
		Update:  chat.BuildOffWorkUpdate(tableName, chatID, offTime, workdays),
	}, nil
}

// InvalidSetOffFromWorkTimeUTCMessage returns the invalid-format reply.
// For example, chat title "Ops" becomes the formatted contract help text for
// invalid setOffFromWorkTimeUTC input.
func InvalidSetOffFromWorkTimeUTCMessage(chatTitle string) string {
	return fmt.Sprintf(`
圍爐區: %s

Error: Invalid format for setOffFromWorkTimeUTC

Format:
/setOffFromWorkTimeUTC {{ 0000-2345, 15 minutes interval }} {{ MON,TUE,WED,THU,FRI,SAT,SUN }}
E.g.:
/setOffFromWorkTimeUTC 1800 MON,TUE,WED,THU,FRI
`, chatTitle)
}

// enabledMessage returns the enable-all-jung reply.
// For example, chat title "Ops" becomes "圍爐區: Ops" plus the enabled notice.
func enabledMessage(chatTitle string) string {
	return fmt.Sprintf(`
圍爐區: %s

Enabled AllJung command`, chatTitle)
}

// disabledMessage returns the disable-all-jung reply.
// For example, chat title "Ops" becomes "圍爐區: Ops" plus the disabled notice.
func disabledMessage(chatTitle string) string {
	return fmt.Sprintf(`
圍爐區: %s

Disabled AllJung command`, chatTitle)
}

// setOffMessage returns the off-work update reply.
// For example, "Ops", "1830", "MON,TUE" becomes the formatted update reply.
func setOffMessage(chatTitle string, offTime string, workdayList string) string {
	return fmt.Sprintf(`
圍爐區: %s

Updated setOffFromWorkTime in UTC: %s %s`, chatTitle, offTime, workdayList)
}

// weekdayToken converts a weekday to its contract token.
// For example, time.Monday becomes "MON".
func weekdayToken(weekday time.Weekday) string {
	switch weekday {
	case time.Sunday:
		return "SUN"
	case time.Monday:
		return "MON"
	case time.Tuesday:
		return "TUE"
	case time.Wednesday:
		return "WED"
	case time.Thursday:
		return "THU"
	case time.Friday:
		return "FRI"
	case time.Saturday:
		return "SAT"
	default:
		return ""
	}
}
