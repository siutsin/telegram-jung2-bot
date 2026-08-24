package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

// chatRepository is the chat persistence surface the service actions need.
type chatRepository interface {
	DueChatIDs(ctx context.Context, tableName string, timestamp time.Time) ([]int64, error)
	Get(ctx context.Context, tableName string, chatID int64) (ChatSetting, bool, error)
	SaveStatistics(ctx context.Context, tableName string, chatID int64, userCount int, messageCount int, now time.Time) error
	Update(ctx context.Context, request UpdateExpression) error
}

type messageRepository interface {
	QueryByChat(ctx context.Context, tableName string, chatID int64, since time.Time) ([]StoredMessage, error)
}

// telegramMessenger is the Telegram surface the service actions need.
type telegramMessenger interface {
	IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageWithOptions(ctx context.Context, chatID int64, text string, options SendMessageOptions) error
}

type queueSender interface {
	SendMessage(ctx context.Context, request queue.SendMessageRequest) error
}

// Service owns the application behaviour behind worker actions.
type Service struct {
	chatMaintainer chatRepository
	chatTable      string
	messageQuerier messageRepository
	messageTable   string
	messenger      telegramMessenger
	nowFunc        func() time.Time
	queueURL       string
	sender         queueSender
	metrics        *Metrics
}

// NewService builds the action service from simple runtime parameters.
func NewService(
	chatMaintainer chatRepository,
	chatTable string,
	messageQuerier messageRepository,
	messageTable string,
	messenger telegramMessenger,
	now func() time.Time,
	queueURL string,
	sender queueSender,
	metrics ...*Metrics,
) Service {
	return Service{
		chatMaintainer: chatMaintainer,
		chatTable:      chatTable,
		messageQuerier: messageQuerier,
		messageTable:   messageTable,
		messenger:      messenger,
		nowFunc:        now,
		queueURL:       queueURL,
		sender:         sender,
		metrics:        firstMetrics(metrics),
	}
}

// AllJung sends the full report when enabled for the chat.
func (service Service) AllJung(ctx context.Context, chatID int64) error {
	row, ok, err := service.chatMaintainer.Get(ctx, service.chatTable, chatID)
	if err != nil {
		return err
	}
	if ok && !row.EnableAllJung {
		return nil
	}

	return service.sendStatistics(ctx, chatID, ReportOptions{})
}

// DisableAllJung updates and replies to the disable command.
func (service Service) DisableAllJung(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
	isAdmin, err := service.messenger.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return err
	}

	change := DisableAllJung(service.chatTable, chatID, chatTitle, isAdmin)
	return service.applySettingChange(ctx, chatID, change)
}

// EnableAllJung updates and replies to the enable command.
func (service Service) EnableAllJung(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
	isAdmin, err := service.messenger.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return err
	}

	change := EnableAllJung(service.chatTable, chatID, chatTitle, isAdmin)
	return service.applySettingChange(ctx, chatID, change)
}

// JungHelp sends the bot help response.
func (service Service) JungHelp(ctx context.Context, chatID int64, chatTitle string) error {
	return service.messenger.SendMessageWithOptions(ctx, chatID, HelpMessage(chatTitle), SendMessageOptions{
		DisableWebPagePreview: true,
		ParseMode:             "markdown",
	})
}

// OffFromWork sends the off-work report.
func (service Service) OffFromWork(ctx context.Context, chatID int64) error {
	return service.sendStatistics(ctx, chatID, ReportOptions{Limit: 10, OffFromWork: true})
}

// OnOffFromWork fans out due off-work actions for one scheduled instant.
func (service Service) OnOffFromWork(ctx context.Context, timeString string) error {
	timestamp, err := ParseScheduledTime(timeString)
	if err != nil {
		return err
	}

	chatIDs, err := service.chatMaintainer.DueChatIDs(ctx, service.chatTable, timestamp)
	if err != nil {
		return err
	}

	producer := queue.NewProducer(service.queueURL, service.sender)
	for _, chatID := range chatIDs {
		err = producer.Enqueue(ctx, BuildOffFromWorkAction(chatID))
		if service.metrics != nil {
			service.metrics.RecordOffWorkReportEnqueue(err)
		}
		if err != nil {
			return fmt.Errorf("enqueue due off-work report: %w", err)
		}
		err = pauseFanOut(ctx, 5*time.Millisecond)
		if err != nil {
			return err
		}
	}

	return nil
}

// SetOffWorkTime updates and replies to the off-work settings command.
func (service Service) SetOffWorkTime(ctx context.Context, input SetOffInput) error {
	isAdmin, err := service.messenger.IsAdmin(ctx, input.ChatID, input.UserID)
	if err != nil {
		return err
	}

	change, err := SetOffFromWorkTimeUTC(service.chatTable, input.ChatID, input.ChatTitle, isAdmin, input.OffTime, input.Workday)
	if err != nil {
		sendErr := service.messenger.SendMessage(
			ctx,
			input.ChatID,
			InvalidSetOffFromWorkTimeUTCMessage(input.ChatTitle),
		)
		if sendErr != nil && !isTelegramStatusError(sendErr) {
			return sendErr
		}

		return nil
	}

	return service.applySettingChange(ctx, input.ChatID, change)
}

// TopDiver sends the reverse ranking report.
func (service Service) TopDiver(ctx context.Context, chatID int64) error {
	return service.sendStatistics(ctx, chatID, ReportOptions{Limit: 10, Reverse: true})
}

// TopTen sends the top-ten report.
func (service Service) TopTen(ctx context.Context, chatID int64) error {
	return service.sendStatistics(ctx, chatID, ReportOptions{Limit: 10})
}

// applySettingChange writes and replies to one admin settings change.
func (service Service) applySettingChange(ctx context.Context, chatID int64, change SettingChange) error {
	if !change.Allowed {
		return nil
	}
	err := service.chatMaintainer.Update(ctx, change.Update)
	if err != nil {
		return err
	}

	err = service.messenger.SendMessage(ctx, chatID, change.Reply)
	if err != nil && isTelegramStatusError(err) {
		return nil
	}

	return err
}

// now returns the configured service clock.
func (service Service) now() time.Time {
	if service.nowFunc == nil {
		return time.Now()
	}

	return service.nowFunc()
}

// pauseFanOut preserves the deployed scheduler pacing between sends.
func pauseFanOut(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// sendStatistics renders, stores counts, and sends one report.
// For example, top-ten options become a rendered report, a saved chat count
// update, and one Telegram send.
func (service Service) sendStatistics(ctx context.Context, chatID int64, options ReportOptions) error {
	now := service.now()
	options.Now = now

	windowDays := options.WindowDays
	if windowDays == 0 {
		windowDays = DefaultWindowDays()
	}

	rows, err := service.messageQuerier.QueryByChat(ctx, service.messageTable, chatID, now.AddDate(0, 0, -windowDays))
	if err != nil {
		return err
	}

	summary, err := GenerateReport(rows, options)
	if errors.Is(err, ErrEmptyRows) {
		slog.Info("skip statistics report for empty chat window", "chatId", chatID)
		return nil
	}
	err = service.chatMaintainer.SaveStatistics(ctx, service.chatTable, chatID, summary.UserCount, summary.MessageCount, now)
	if err != nil {
		return err
	}

	err = service.messenger.SendMessage(ctx, chatID, summary.Report)
	if err != nil {
		if isTelegramStatusError(err) {
			return nil
		}
		return err
	}

	return nil
}

// isTelegramStatusError reports whether err is a Telegram API 4xx or 5xx error.
// For example, "telegram API returned HTTP 429" matches, while "timeout" does
// not.
func isTelegramStatusError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "telegram API returned HTTP 4") ||
		strings.Contains(err.Error(), "telegram API returned HTTP 5")
}
