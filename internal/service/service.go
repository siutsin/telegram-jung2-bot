// Package service owns the bot actions executed from queue messages.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	contractdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/statistics"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

// ChatStore is the chat persistence surface the service actions need.
type ChatStore interface {
	DueChatIDs(ctx context.Context, tableName string, timestamp time.Time) ([]int64, error)
	Get(ctx context.Context, tableName string, chatID int64) (chat.Row, bool, error)
	SaveStatistics(ctx context.Context, tableName string, chatID int64, userCount int, messageCount int, now time.Time) error
	Update(ctx context.Context, request chat.UpdateExpression) error
}

// Messenger is the Telegram surface the service actions need.
type Messenger interface {
	IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageWithOptions(ctx context.Context, chatID int64, text string, options telegram.SendMessageOptions) error
}

// Service owns the application behaviour behind worker actions.
type Service struct {
	ChatStore         ChatStore
	ChatTable         string
	MessageRepository message.Repository
	Messenger         Messenger
	Now               func() time.Time
	QueueURL          string
	Sender            queue.Sender
}

// AllJung sends the full report when enabled for the chat.
func (service Service) AllJung(ctx context.Context, chatID int64) error {
	row, ok, err := service.ChatStore.Get(ctx, service.ChatTable, chatID)
	if err != nil {
		return err
	}
	if ok && row.EnableAllJung != nil && !*row.EnableAllJung {
		return nil
	}

	return service.sendStatistics(ctx, chatID, statistics.Options{})
}

// DisableAllJung updates and replies to the disable command.
func (service Service) DisableAllJung(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
	isAdmin, err := service.Messenger.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return err
	}

	change := schedule.DisableAllJung(service.ChatTable, chatID, chatTitle, isAdmin)
	return service.applySettingChange(ctx, chatID, change)
}

// EnableAllJung updates and replies to the enable command.
func (service Service) EnableAllJung(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
	isAdmin, err := service.Messenger.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return err
	}

	change := schedule.EnableAllJung(service.ChatTable, chatID, chatTitle, isAdmin)
	return service.applySettingChange(ctx, chatID, change)
}

// JungHelp sends the bot help response.
func (service Service) JungHelp(ctx context.Context, chatID int64, chatTitle string) error {
	return service.Messenger.SendMessageWithOptions(ctx, chatID, statistics.HelpMessage(chatTitle), telegram.SendMessageOptions{
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

	chatIDs, err := service.ChatStore.DueChatIDs(ctx, service.ChatTable, timestamp)
	if err != nil {
		return err
	}

	producer := queue.Producer{
		QueueURL: service.QueueURL,
		Sender:   service.Sender,
	}
	for _, chatID := range chatIDs {
		if err := producer.Enqueue(ctx, schedule.BuildOffFromWorkAction(chatID)); err != nil {
			return fmt.Errorf("enqueue due off-work report: %w", err)
		}
		if err := pauseFanOut(ctx, 5*time.Millisecond); err != nil {
			return err
		}
	}

	return nil
}

// SetOffWorkTime updates and replies to the off-work settings command.
func (service Service) SetOffWorkTime(ctx context.Context, input worker.SetOffInput) error {
	isAdmin, err := service.Messenger.IsAdmin(ctx, input.ChatID, input.UserID)
	if err != nil {
		return err
	}

	change, err := schedule.SetOffFromWorkTimeUTC(service.ChatTable, input.ChatID, input.ChatTitle, isAdmin, input.OffTime, input.Workday)
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
	if err := service.ChatStore.Update(ctx, change.Update); err != nil {
		return err
	}

	return service.Messenger.SendMessage(ctx, chatID, change.Reply)
}

// now returns the configured service clock.
func (service Service) now() time.Time {
	if service.Now == nil {
		return time.Now()
	}

	return service.Now()
}

// parseScheduledTime parses the scheduler time string.
func parseScheduledTime(raw string) (time.Time, error) {
	if timestamp, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return timestamp, nil
	}

	timestamp, err := time.Parse(time.RFC3339, raw)
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
func (service Service) sendStatistics(ctx context.Context, chatID int64, options statistics.Options) error {
	now := service.now()
	options.Now = now

	rows, err := service.MessageRepository.QueryByChat(ctx, chatID, now.AddDate(0, 0, -7), now)
	if err != nil {
		return err
	}

	summary, err := statistics.GenerateReport(rows, options)
	if err != nil {
		return err
	}
	if err := service.ChatStore.SaveStatistics(ctx, service.ChatTable, chatID, summary.UserCount, summary.MessageCount, now); err != nil {
		return err
	}

	if err := service.Messenger.SendMessage(ctx, chatID, summary.Report); err != nil {
		if isTelegramStatusError(err) {
			return nil
		}
		return err
	}

	return nil
}

// isTelegramStatusError reports whether err is a Telegram API 4xx or 5xx error.
func isTelegramStatusError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "telegram API returned HTTP 4") ||
		strings.Contains(err.Error(), "telegram API returned HTTP 5")
}

var _ ChatStore = contractdynamodb.ChatClient{}
