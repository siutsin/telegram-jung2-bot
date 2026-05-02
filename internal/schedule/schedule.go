// Package schedule owns off-work scheduling and admin setting domain behaviour.
package schedule

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

type Window struct {
	OffTime string
	Weekday string
}

type SettingChange struct {
	Allowed bool
	Reply   string
	Update  chat.UpdateExpression
}

type ChatRepository interface {
	ListEnabled(ctx context.Context) ([]chat.Settings, error)
}

type Enqueuer interface {
	Enqueue(ctx context.Context, action queue.Action) error
}

type Scheduler interface {
	Sync(ctx context.Context, settings chat.Settings) error
}

type Service struct {
	Chats     ChatRepository
	Enqueuer  Enqueuer
	Scheduler Scheduler
}

func (service Service) SyncChat(ctx context.Context, settings chat.Settings) error {
	if service.Scheduler == nil {
		return fmt.Errorf("scheduler is required")
	}
	if err := service.Scheduler.Sync(ctx, settings); err != nil {
		return fmt.Errorf("sync chat schedule: %w", err)
	}

	return nil
}

func (service Service) HandleDueReport(ctx context.Context, timestamp time.Time) error {
	if service.Chats == nil {
		return fmt.Errorf("chat repository is required")
	}
	if service.Enqueuer == nil {
		return fmt.Errorf("enqueuer is required")
	}
	rows, err := service.Chats.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("list due chats: %w", err)
	}
	for _, chatID := range DueChatIDs(rows, timestamp) {
		if err := service.Enqueuer.Enqueue(ctx, BuildOffFromWorkAction(chatID)); err != nil {
			return fmt.Errorf("enqueue due off-work report: %w", err)
		}
	}

	return nil
}

func WindowFromTime(timestamp time.Time) Window {
	return Window{
		OffTime: timestamp.UTC().Format("1504"),
		Weekday: weekdayToken(timestamp.UTC().Weekday()),
	}
}

func DueChatIDs(rows []chat.Settings, timestamp time.Time) []int64 {
	window := WindowFromTime(timestamp)
	due := chat.FilterDue(rows, window.OffTime, window.Weekday)
	chatIDs := make([]int64, 0, len(due))
	for _, settings := range due {
		chatIDs = append(chatIDs, settings.ChatID)
	}

	return chatIDs
}

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

func enabledMessage(chatTitle string) string {
	return fmt.Sprintf(`
圍爐區: %s

Enabled AllJung command`, chatTitle)
}

func disabledMessage(chatTitle string) string {
	return fmt.Sprintf(`
圍爐區: %s

Disabled AllJung command`, chatTitle)
}

func setOffMessage(chatTitle string, offTime string, workdayList string) string {
	return fmt.Sprintf(`
圍爐區: %s

Updated setOffFromWorkTime in UTC: %s %s`, chatTitle, offTime, workdayList)
}

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
