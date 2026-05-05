// Package service owns the bot actions executed from queue messages.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/statistics"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

// chatRepository is the chat persistence surface the service actions need.
type chatRepository interface {
	DueChatIDs(ctx context.Context, tableName string, timestamp time.Time) ([]int64, error)
	Get(ctx context.Context, tableName string, chatID int64) (chat.ChatSetting, bool, error)
	SaveStatistics(ctx context.Context, tableName string, chatID int64, userCount int, messageCount int, now time.Time) error
	Update(ctx context.Context, request chat.UpdateExpression) error
}

type messageRepository interface {
	QueryByChat(ctx context.Context, tableName string, chatID int64, since time.Time) ([]message.Message, error)
}

// telegramMessenger is the Telegram surface the service actions need.
type telegramMessenger interface {
	IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageWithOptions(ctx context.Context, chatID int64, text string, options telegram.SendMessageOptions) error
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
}

// New builds the action service from simple runtime parameters.
func New(
	chatMaintainer chatRepository,
	chatTable string,
	messageQuerier messageRepository,
	messageTable string,
	messenger telegramMessenger,
	now func() time.Time,
	queueURL string,
	sender queueSender,
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

	return service.sendStatistics(ctx, chatID, statistics.Options{})
}

// DisableAllJung updates and replies to the disable command.
func (service Service) DisableAllJung(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
	isAdmin, err := service.messenger.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return err
	}

	change := schedule.DisableAllJung(service.chatTable, chatID, chatTitle, isAdmin)
	return service.applySettingChange(ctx, chatID, change)
}

// EnableAllJung updates and replies to the enable command.
func (service Service) EnableAllJung(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
	isAdmin, err := service.messenger.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return err
	}

	change := schedule.EnableAllJung(service.chatTable, chatID, chatTitle, isAdmin)
	return service.applySettingChange(ctx, chatID, change)
}

// JungHelp sends the bot help response.
func (service Service) JungHelp(ctx context.Context, chatID int64, chatTitle string) error {
	return service.messenger.SendMessageWithOptions(ctx, chatID, statistics.HelpMessage(chatTitle), telegram.SendMessageOptions{
		DisableWebPagePreview: true,
		ParseMode:             "markdown",
	})
}

// OffFromWork sends the off-work report.
func (service Service) OffFromWork(ctx context.Context, chatID int64) error {
	return service.sendStatistics(ctx, chatID, statistics.Options{Limit: 10, OffFromWork: true})
}

// OnOffFromWork fans out due off-work actions for one scheduled instant.
func (service Service) OnOffFromWork(ctx context.Context, timeString string) error {
	timestamp, err := parseScheduledTime(timeString)
	if err != nil {
		return err
	}

	chatIDs, err := service.chatMaintainer.DueChatIDs(ctx, service.chatTable, timestamp)
	if err != nil {
		return err
	}

	producer := queue.NewProducer(service.queueURL, service.sender)
	for _, chatID := range chatIDs {
		err = producer.Enqueue(ctx, schedule.BuildOffFromWorkAction(chatID))
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
func (service Service) SetOffWorkTime(ctx context.Context, input worker.SetOffInput) error {
	isAdmin, err := service.messenger.IsAdmin(ctx, input.ChatID, input.UserID)
	if err != nil {
		return err
	}

	change, err := schedule.SetOffFromWorkTimeUTC(service.chatTable, input.ChatID, input.ChatTitle, isAdmin, input.OffTime, input.Workday)
	if err != nil {
		return err
	}

	return service.applySettingChange(ctx, input.ChatID, change)
}

// TopDiver sends the reverse ranking report.
func (service Service) TopDiver(ctx context.Context, chatID int64) error {
	return service.sendStatistics(ctx, chatID, statistics.Options{Limit: 10, Reverse: true})
}

// TopTen sends the top-ten report.
func (service Service) TopTen(ctx context.Context, chatID int64) error {
	return service.sendStatistics(ctx, chatID, statistics.Options{Limit: 10})
}

// applySettingChange writes and replies to one admin settings change.
func (service Service) applySettingChange(ctx context.Context, chatID int64, change schedule.SettingChange) error {
	if !change.Allowed {
		return nil
	}
	err := service.chatMaintainer.Update(ctx, change.Update)
	if err != nil {
		return err
	}

	return service.messenger.SendMessage(ctx, chatID, change.Reply)
}

// now returns the configured service clock.
func (service Service) now() time.Time {
	if service.nowFunc == nil {
		return time.Now()
	}

	return service.nowFunc()
}

// parseScheduledTime parses the scheduler time string.
// For example, "2025-01-06T18:30:00Z" becomes the matching time.Time.
func parseScheduledTime(raw string) (time.Time, error) {
	timestamp, err := time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return timestamp, nil
	}

	timestamp, err = time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse scheduled time: %w", err)
	}

	return timestamp, nil
}

// pauseFanOut preserves the reference scheduler pacing between sends.
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
func (service Service) sendStatistics(ctx context.Context, chatID int64, options statistics.Options) error {
	now := service.now()
	options.Now = now

	rows, err := service.messageQuerier.QueryByChat(ctx, service.messageTable, chatID, now.AddDate(0, 0, -7))
	if err != nil {
		return err
	}

	summary, err := statistics.GenerateReport(rows, options)
	if err != nil {
		return err
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

var _ chatRepository = dynamodb.NewChatClient(nil)
