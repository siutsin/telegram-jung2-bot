// Package worker owns queue action dispatch without binding to an SQS SDK.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
)

type Handlers struct {
	JungHelp       func(ctx context.Context, chatID int64, chatTitle string) error
	TopTen         func(ctx context.Context, chatID int64) error
	TopDiver       func(ctx context.Context, chatID int64) error
	AllJung        func(ctx context.Context, chatID int64) error
	OffFromWork    func(ctx context.Context, chatID int64) error
	EnableAllJung  func(ctx context.Context, chatID int64, chatTitle string, userID int64) error
	DisableAllJung func(ctx context.Context, chatID int64, chatTitle string, userID int64) error
	SetOffWorkTime func(ctx context.Context, input schedule.SetOffInput) error
	OnOffFromWork  func(ctx context.Context, timeString string) error
}

type queueDeleter interface {
	Delete(ctx context.Context, request queue.DeleteMessageRequest) error
}

type queueReceiver interface {
	ReceiveMessage(ctx context.Context, request queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error)
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
func NewPollingWorker(queueURL string, receiver queueReceiver, deleter queueDeleter, handlers Handlers) (pollingWorker, error) {
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
	action := queue.DecodeMessage(raw)
	dispatchErr := dispatch(ctx, action, handlers)
	if dispatchErr != nil {
		slog.Error("queue message dispatch failed", "action", action.Name, "err", dispatchErr)
		if !isPermanentDispatchError(dispatchErr) {
			return dispatchErr
		}
	} else {
		err := deleteProcessedMessage(ctx, deleter, queueURL, raw, action.Name)
		if err != nil {
			return err
		}

		return nil
	}

	err := deleteProcessedMessage(ctx, deleter, queueURL, raw, action.Name)
	if err != nil {
		return err
	}

	return nil
}

// deleteProcessedMessage deletes a successfully handled queue message.
func deleteProcessedMessage(ctx context.Context, deleter queueDeleter, queueURL string, raw queue.RawMessage, actionName string) error {
	deleteCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	err := deleter.Delete(deleteCtx, queue.DeleteMessageRequest{
		QueueURL:      queueURL,
		ReceiptHandle: raw.ReceiptHandle,
	})
	if err != nil {
		slog.Error("queue message delete failed", "action", actionName, "err", err)
		return err
	}

	return nil
}

// isPermanentDispatchError reports malformed queue payloads that should not retry.
func isPermanentDispatchError(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	return strings.HasPrefix(message, "missing ") ||
		strings.HasPrefix(message, "invalid ") ||
		strings.HasPrefix(message, "missing handler for ")
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

		chatID, err := requiredChatID(action)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, chatID)
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

		chatID, err := requiredChatID(action)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, chatID, action.Attributes["chatTitle"])
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

		chatID, err := requiredChatID(action)
		if err != nil {
			return err
		}
		userID, err := requiredUserID(action)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, chatID, action.Attributes["chatTitle"], userID)
	}
}

// withSetOffInput builds a dispatcher for off-work schedule updates.
// For example, action attributes become SetOffInput{ChatID, ChatTitle, UserID,
// OffTime, Workday}.
func withSetOffInput(handler func(ctx context.Context, input schedule.SetOffInput) error, actionName string) actionDispatcher {
	return func(ctx context.Context, action queue.Action) error {
		requiredHandler, err := requireHandler(handler, actionName)
		if err != nil {
			return err
		}

		chatID, err := requiredChatID(action)
		if err != nil {
			return err
		}
		userID, err := requiredUserID(action)
		if err != nil {
			return err
		}

		return requiredHandler(ctx, schedule.SetOffInput{
			ChatID:    chatID,
			ChatTitle: action.Attributes["chatTitle"],
			UserID:    userID,
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

		timeString := action.Attributes["timeString"]
		if timeString == "" {
			return fmt.Errorf("missing timeString for %s", actionName)
		}

		return requiredHandler(ctx, timeString)
	}
}

// requiredChatID parses a required chatId attribute.
// For example, Attributes["chatId"]="42" becomes 42.
func requiredChatID(action queue.Action) (int64, error) {
	return requiredIntAttribute(action, "chatId")
}

// requiredUserID parses a required userId attribute.
// For example, Attributes["userId"]="7" becomes 7.
func requiredUserID(action queue.Action) (int64, error) {
	return requiredIntAttribute(action, "userId")
}

// requiredIntAttribute parses a required decimal int64 action attribute.
// For example, chatId "42" becomes 42, while a missing key returns an error.
func requiredIntAttribute(action queue.Action, key string) (int64, error) {
	raw, ok := action.Attributes[key]
	if !ok || raw == "" {
		return 0, fmt.Errorf("missing %s for %s", key, action.Name)
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s for %s: %w", key, action.Name, err)
	}

	return value, nil
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
