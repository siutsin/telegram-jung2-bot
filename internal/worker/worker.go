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

type actionDispatcher func(ctx context.Context, action queue.Action) error

// Run polls the queue until the context is cancelled.
func (worker PollingWorker) Run(ctx context.Context) error {
	if worker.Deleter == nil {
		return fmt.Errorf("deleter is required")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if err := worker.Consumer.Poll(ctx, func(ctx context.Context, message queue.RawMessage) error {
			return ProcessMessage(ctx, worker.QueueURL, message, worker.Handlers, worker.Deleter)
		}); err != nil {
			return err
		}
	}
}

// Dispatch routes an action to its handler.
func Dispatch(ctx context.Context, action queue.Action, handlers Handlers) error {
	dispatcher, ok := actionDispatchers(handlers)[action.Name]
	if !ok {
		return nil
	}

	return dispatcher(ctx, action)
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

// actionDispatchers returns the queue action dispatch table.
func actionDispatchers(handlers Handlers) map[string]actionDispatcher {
	return map[string]actionDispatcher{
		queue.ActionJungHelp:       withChatIDAndTitle(handlers.JungHelp, queue.ActionJungHelp),
		queue.ActionTopTen:         withChatID(handlers.TopTen, queue.ActionTopTen),
		queue.ActionTopDiver:       withChatID(handlers.TopDiver, queue.ActionTopDiver),
		queue.ActionAllJung:        withChatID(handlers.AllJung, queue.ActionAllJung),
		queue.ActionOffFromWork:    withChatID(handlers.OffFromWork, queue.ActionOffFromWork),
		queue.ActionEnableAllJung:  withAdminFields(handlers.EnableAllJung, queue.ActionEnableAllJung),
		queue.ActionDisableAllJung: withAdminFields(handlers.DisableAllJung, queue.ActionDisableAllJung),
		queue.ActionSetOffWorkTime: withSetOffInput(handlers.SetOffWorkTime, queue.ActionSetOffWorkTime),
		queue.ActionOnOffFromWork:  withTimeString(handlers.OnOffFromWork, queue.ActionOnOffFromWork),
	}
}

// withChatID builds a dispatcher for actions that only need a chat ID.
func withChatID(handler func(ctx context.Context, chatID int64) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, chatID(action))
	}
}

// withChatIDAndTitle builds a dispatcher for actions that need chat metadata.
func withChatIDAndTitle(handler func(ctx context.Context, chatID int64, chatTitle string) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, chatID(action), action.Attributes["chatTitle"])
	}
}

// withAdminFields builds a dispatcher for admin-gated chat actions.
func withAdminFields(handler func(ctx context.Context, chatID int64, chatTitle string, userID int64) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, chatID(action), action.Attributes["chatTitle"], userID(action))
	}
}

// withSetOffInput builds a dispatcher for off-work schedule updates.
func withSetOffInput(handler func(ctx context.Context, input SetOffInput) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, SetOffInput{
			ChatID:    chatID(action),
			ChatTitle: action.Attributes["chatTitle"],
			UserID:    userID(action),
			OffTime:   action.Attributes["offTime"],
			Workday:   action.Attributes["workday"],
		})
	}
}

// withTimeString builds a dispatcher for scheduled off-work fan-out.
func withTimeString(handler func(ctx context.Context, timeString string) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, action.Attributes["timeString"])
	}
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
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}

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
