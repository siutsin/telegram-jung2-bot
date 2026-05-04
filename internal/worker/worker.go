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

type queueDeleter interface {
	Delete(ctx context.Context, request queue.DeleteMessageRequest) error
}

type queueConsumer interface {
	Poll(ctx context.Context, handler func(context.Context, queue.RawMessage) error) error
}

type pollingWorker struct {
	consumer queueConsumer
	queueURL string
	handlers Handlers
	deleter  queueDeleter
}

type actionDispatcher func(ctx context.Context, action queue.Action) error

// NewPollingWorker builds a queue worker from the configured queue contracts.
func NewPollingWorker(queueURL string, receiver queue.Receiver, deleter queueDeleter, handlers Handlers) (pollingWorker, error) {
	if receiver == nil {
		return pollingWorker{}, fmt.Errorf("queue receiver is required")
	}
	if deleter == nil {
		return pollingWorker{}, fmt.Errorf("queue deleter is required")
	}

	return pollingWorker{
		consumer: queue.NewConsumer(queueURL, receiver),
		queueURL: queueURL,
		handlers: handlers,
		deleter:  deleter,
	}, nil
}

// Run polls the queue until the context is cancelled.
func (worker pollingWorker) Run(ctx context.Context) error {
	if worker.deleter == nil {
		return fmt.Errorf("deleter is required")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		err := worker.consumer.Poll(ctx, func(pollCtx context.Context, message queue.RawMessage) error {
			return processMessage(pollCtx, worker.queueURL, message, worker.handlers, worker.deleter)
		})
		if err != nil {
			return err
		}
	}
}

// dispatch routes an action to its handler.
// For example, Action{Name: "topTen"} is sent to handlers.TopTen.
func dispatch(ctx context.Context, action queue.Action, handlers Handlers) error {
	dispatcher, ok := actionDispatchers(handlers)[action.Name]
	if !ok {
		return nil
	}

	return dispatcher(ctx, action)
}

// processMessage decodes, dispatches, and deletes a queue message.
// For example, one raw SQS message becomes queue.DecodeMessage(raw), one handler
// call, and one delete request.
func processMessage(ctx context.Context, queueURL string, raw queue.RawMessage, handlers Handlers, deleter queueDeleter) error {
	action, err := queue.DecodeMessage(raw)
	if err != nil {
		return err
	}
	dispatchErr := dispatch(ctx, action, handlers)
	if dispatchErr != nil {
		return dispatchErr
	}
	err = deleter.Delete(ctx, queue.DeleteMessageRequest{
		QueueURL:      queueURL,
		ReceiptHandle: raw.ReceiptHandle,
	})
	if err != nil {
		return fmt.Errorf("delete SQS message: %w", err)
	}

	return nil
}

// actionDispatchers returns the queue action dispatch table.
// For example, "jungHelp" maps to the dispatcher built by withChatIDAndTitle.
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
// For example, Attributes["chatId"]="42" becomes handler(ctx, 42).
func withChatID(handler func(ctx context.Context, chatID int64) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, actionChatID(action))
	}
}

// withChatIDAndTitle builds a dispatcher for actions that need chat metadata.
// For example, chatId "42" and chatTitle "Ops" become handler(ctx, 42, "Ops").
func withChatIDAndTitle(handler func(ctx context.Context, chatID int64, chatTitle string) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, actionChatID(action), action.Attributes["chatTitle"])
	}
}

// withAdminFields builds a dispatcher for admin-gated chat actions.
// For example, chatId "42", chatTitle "Ops", and userId "7" become
// handler(ctx, 42, "Ops", 7).
func withAdminFields(handler func(ctx context.Context, chatID int64, chatTitle string, userID int64) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, actionChatID(action), action.Attributes["chatTitle"], actionUserID(action))
	}
}

// withSetOffInput builds a dispatcher for off-work schedule updates.
// For example, action attributes become SetOffInput{ChatID, ChatTitle, UserID,
// OffTime, Workday}.
func withSetOffInput(handler func(ctx context.Context, input SetOffInput) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, SetOffInput{
			ChatID:    actionChatID(action),
			ChatTitle: action.Attributes["chatTitle"],
			UserID:    actionUserID(action),
			OffTime:   action.Attributes["offTime"],
			Workday:   action.Attributes["workday"],
		})
	}
}

// withTimeString builds a dispatcher for scheduled off-work fan-out.
// For example, Attributes["timeString"]="2025-01-06T18:30:00Z" is passed
// straight to the handler.
func withTimeString(handler func(ctx context.Context, timeString string) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, action.Attributes["timeString"])
	}
}

// actionChatID parses the chat ID from action attributes.
// For example, Attributes["chatId"]="42" becomes 42.
func actionChatID(action queue.Action) int64 {
	return parseInt(action.Attributes["chatId"])
}

// actionUserID parses the user ID from action attributes.
// For example, Attributes["userId"]="7" becomes 7.
func actionUserID(action queue.Action) int64 {
	return parseInt(action.Attributes["userId"])
}

// parseInt parses a decimal int64 and falls back to zero.
// For example, "42" becomes 42, while "bad" becomes 0.
func parseInt(raw string) int64 {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}

	return value
}

// requireHandler returns a configured handler for an action.
func requireHandler[T any](handler T, actionName string) (T, error) {
	value := reflect.ValueOf(handler)
	if !value.IsValid() || value.IsNil() {
		var zero T
		return zero, fmt.Errorf("missing handler for %s", actionName)
	}

	return handler, nil
}
