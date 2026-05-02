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

// Run polls the queue until the context is cancelled.
func (worker PollingWorker) Run(ctx context.Context) error {
	if worker.Deleter == nil {
		return fmt.Errorf("deleter is required")
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if err := worker.Consumer.Poll(ctx, func(ctx context.Context, message queue.RawMessage) error {
			if err := ProcessMessage(ctx, worker.QueueURL, message, worker.Handlers, worker.Deleter); err != nil {
				return nil
			}
			return nil
		}); err != nil {
			return err
		}
	}
}

// Dispatch routes an action to its handler.
func Dispatch(ctx context.Context, action queue.Action, handlers Handlers) error {
	switch action.Name {
	case queue.ActionJungHelp:
		handler, err := requireHandler(handlers.JungHelp, action.Name)
		if err != nil {
			return err
		}
		return handler(ctx, chatID(action), action.Attributes["chatTitle"])
	case queue.ActionTopTen:
		handler, err := requireHandler(handlers.TopTen, action.Name)
		if err != nil {
			return err
		}
		return handler(ctx, chatID(action))
	case queue.ActionTopDiver:
		handler, err := requireHandler(handlers.TopDiver, action.Name)
		if err != nil {
			return err
		}
		return handler(ctx, chatID(action))
	case queue.ActionAllJung:
		handler, err := requireHandler(handlers.AllJung, action.Name)
		if err != nil {
			return err
		}
		return handler(ctx, chatID(action))
	case queue.ActionOffFromWork:
		handler, err := requireHandler(handlers.OffFromWork, action.Name)
		if err != nil {
			return err
		}
		return handler(ctx, chatID(action))
	case queue.ActionEnableAllJung:
		handler, err := requireHandler(handlers.EnableAllJung, action.Name)
		if err != nil {
			return err
		}
		return handler(ctx, chatID(action), action.Attributes["chatTitle"], userID(action))
	case queue.ActionDisableAllJung:
		handler, err := requireHandler(handlers.DisableAllJung, action.Name)
		if err != nil {
			return err
		}
		return handler(ctx, chatID(action), action.Attributes["chatTitle"], userID(action))
	case queue.ActionSetOffWorkTime:
		handler, err := requireHandler(handlers.SetOffWorkTime, action.Name)
		if err != nil {
			return err
		}
		return handler(ctx, SetOffInput{
			ChatID:    chatID(action),
			ChatTitle: action.Attributes["chatTitle"],
			UserID:    userID(action),
			OffTime:   action.Attributes["offTime"],
			Workday:   action.Attributes["workday"],
		})
	case queue.ActionOnOffFromWork:
		handler, err := requireHandler(handlers.OnOffFromWork, action.Name)
		if err != nil {
			return err
		}
		return handler(ctx, action.Attributes["timeString"])
	default:
		return nil
	}
}

// ProcessMessage decodes, dispatches, and deletes a queue message.
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

// chatID parses the chat ID from action attributes.
func chatID(action queue.Action) int64 {
	return parseInt(action.Attributes["chatId"])
}

// userID parses the user ID from action attributes.
func userID(action queue.Action) int64 {
	return parseInt(action.Attributes["userId"])
}

// parseInt parses a decimal int64 and falls back to zero.
func parseInt(raw string) int64 {
	value, _ := strconv.ParseInt(raw, 10, 64)
	return value
}

// requireHandler returns a configured handler for an action.
func requireHandler[T any](handler T, action string) (T, error) {
	value := reflect.ValueOf(handler)
	if !value.IsValid() || value.IsNil() {
		var zero T
		return zero, fmt.Errorf("missing handler for %s", action)
	}

	return handler, nil
}
