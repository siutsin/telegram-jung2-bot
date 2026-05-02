// Package worker owns queue action dispatch without binding to an SQS SDK.
package worker

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

type Handlers struct {
	JungHelp       func(ctx context.Context, chatID int64, chatTitle string) error
	TopTen         func(ctx context.Context, chatID int64) error
	TopDiver       func(ctx context.Context, chatID int64) error
	AllJung        func(ctx context.Context, chatID int64) error
	OffFromWork    func(ctx context.Context, chatID int64) error
	EnableAllJung  func(ctx context.Context, chatID int64, chatTitle string, userID int64) error
	DisableAllJung func(ctx context.Context, chatID int64, chatTitle string, userID int64) error
	SetOffWorkTime func(ctx context.Context, input SetOffInput) error
	OnOffFromWork  func(ctx context.Context, timeString string) error
}

type SetOffInput struct {
	ChatID    int64
	ChatTitle string
	UserID    int64
	OffTime   string
	Workday   string
}

type Deleter interface {
	Delete(ctx context.Context, request queue.DeleteMessageRequest) error
}

type PollingWorker struct {
	Consumer queue.Consumer
	QueueURL string
	Handlers Handlers
	Deleter  Deleter
}

func (worker PollingWorker) Run(ctx context.Context) error {
	if worker.Deleter == nil {
		return fmt.Errorf("deleter is required")
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if err := worker.Consumer.Poll(ctx, func(ctx context.Context, message queue.RawMessage) error {
			return ProcessMessage(ctx, worker.QueueURL, message, worker.Handlers, worker.Deleter)
		}); err != nil {
			return err
		}
	}
}

func Dispatch(ctx context.Context, action queue.Action, handlers Handlers) error {
	switch action.Name {
	case queue.ActionJungHelp:
		return requireHandler(handlers.JungHelp, action.Name)(ctx, chatID(action), action.Attributes["chatTitle"])
	case queue.ActionTopTen:
		return requireHandler(handlers.TopTen, action.Name)(ctx, chatID(action))
	case queue.ActionTopDiver:
		return requireHandler(handlers.TopDiver, action.Name)(ctx, chatID(action))
	case queue.ActionAllJung:
		return requireHandler(handlers.AllJung, action.Name)(ctx, chatID(action))
	case queue.ActionOffFromWork:
		return requireHandler(handlers.OffFromWork, action.Name)(ctx, chatID(action))
	case queue.ActionEnableAllJung:
		return requireHandler(handlers.EnableAllJung, action.Name)(ctx, chatID(action), action.Attributes["chatTitle"], userID(action))
	case queue.ActionDisableAllJung:
		return requireHandler(handlers.DisableAllJung, action.Name)(ctx, chatID(action), action.Attributes["chatTitle"], userID(action))
	case queue.ActionSetOffWorkTime:
		return requireHandler(handlers.SetOffWorkTime, action.Name)(ctx, SetOffInput{
			ChatID:    chatID(action),
			ChatTitle: action.Attributes["chatTitle"],
			UserID:    userID(action),
			OffTime:   action.Attributes["offTime"],
			Workday:   action.Attributes["workday"],
		})
	case queue.ActionOnOffFromWork:
		return requireHandler(handlers.OnOffFromWork, action.Name)(ctx, action.Attributes["timeString"])
	default:
		return fmt.Errorf("unsupported queue action %q", action.Name)
	}
}

func ProcessMessage(ctx context.Context, queueURL string, raw queue.RawMessage, handlers Handlers, deleter Deleter) error {
	action, err := queue.DecodeMessage(raw)
	if err != nil {
		return err
	}
	if err := Dispatch(ctx, action, handlers); err != nil {
		return err
	}
	if err := deleter.Delete(ctx, queue.BuildDeleteMessageRequest(queueURL, raw)); err != nil {
		return fmt.Errorf("delete SQS message: %w", err)
	}

	return nil
}

func chatID(action queue.Action) int64 {
	return parseInt(action.Attributes["chatId"])
}

func userID(action queue.Action) int64 {
	return parseInt(action.Attributes["userId"])
}

func parseInt(raw string) int64 {
	value, _ := strconv.ParseInt(raw, 10, 64)
	return value
}

func requireHandler[T any](handler T, action string) T {
	if reflect.ValueOf(handler).IsNil() {
		panic(fmt.Sprintf("missing handler for %s", action))
	}

	return handler
}
