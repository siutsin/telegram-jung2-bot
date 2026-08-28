package bot

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -destination=worker_mock_test.go -package=bot -mock_names queueReceiver=MockQueueReceiver . queueReceiver"

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

// ErrPermanentDispatch marks queue payload or handler wiring errors that should
// not retry.
var ErrPermanentDispatch = errors.New("permanent queue dispatch error")

// maxQueueReceives is how many times a transient dispatch may run before the
// worker deletes the message. Queue visibility is 600s, so 5 receives is
// about 50 minutes of retries.
const maxQueueReceives = 5

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
	metrics  *Metrics
}

type actionDispatcher func(ctx context.Context, action queue.Action) error

// NewPollingWorker builds a queue worker from the configured queue contracts.
func NewPollingWorker(queueURL string, receiver queueReceiver, deleter queueDeleter, handlers Handlers, metrics ...*Metrics) (pollingWorker, error) {
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
		metrics:  firstMetrics(metrics),
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
			return worker.processMessage(pollCtx, message)
		})
		if err != nil {
			return err
		}
	}
}

// processMessage records the result of one worker action before returning it.
// It decodes raw once and reuses the result, since decoding twice per message
// would double SQS message-attribute parsing for no benefit.
func (worker pollingWorker) processMessage(ctx context.Context, raw queue.RawMessage) error {
	started := time.Now()
	action := queue.DecodeMessage(raw)
	worker.recordQueueWait(action, started)
	actionName, outcome, err := processDecodedMessageResult(ctx, worker.queueURL, raw, action, worker.handlers, worker.deleter)
	if worker.metrics != nil {
		worker.metrics.RecordWorkerAction(actionName, outcome, time.Since(started))
	}

	return err
}

// recordQueueWait measures pickup lag, clamping to zero when the producer and
// worker clocks disagree, since a raw negative sample would silently corrupt
// the histogram sum. A missing or malformed enqueue time is tolerated too, as
// neither case should ever block message processing.
func (worker pollingWorker) recordQueueWait(action queue.Action, pickedUp time.Time) {
	if worker.metrics == nil {
		return
	}

	enqueuedAt, ok := parseEnqueuedAt(action.Attributes)
	if !ok {
		return
	}
	wait := pickedUp.Sub(enqueuedAt)
	if wait < 0 {
		slog.Warn("queue wait was negative, clamping to zero", "action", action.Name)
		wait = 0
	}
	worker.metrics.RecordQueueWait(workerActionName(action.Name), wait)
}

// parseEnqueuedAt reads the enqueue timestamp, reporting false (never an
// error) so an older or foreign message just skips the queue-wait sample.
// For example, Attributes["enqueuedAt"] = "2025-01-06T18:30:00Z" becomes that
// timestamp, while a missing or malformed value reports false.
func parseEnqueuedAt(attributes map[string]string) (time.Time, bool) {
	raw, ok := attributes[queue.EnqueuedAtAttribute]
	if !ok || raw == "" {
		return time.Time{}, false
	}

	enqueuedAt, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, false
	}

	return enqueuedAt, true
}

// dispatch routes an action to its handler.
// For example, Action{Name: "topTen"} is sent to handlers.TopTen.
func dispatch(ctx context.Context, action queue.Action, handlers Handlers) error {
	if action.Name == "" {
		return permanentDispatchError("missing action name")
	}

	dispatcher, ok := actionDispatchers(handlers)[action.Name]
	if !ok {
		slog.Warn("queue message has unsupported action", "action", action.Name)
		return permanentDispatchError("unsupported action %s", action.Name)
	}

	return dispatcher(ctx, action)
}

// processDecodedMessageResult dispatches, deletes, and classifies a queue
// message whose action was already decoded, so the caller that also needs
// the decoded action for queue-wait metrics only decodes it once.
func processDecodedMessageResult(ctx context.Context, queueURL string, raw queue.RawMessage, action queue.Action, handlers Handlers, deleter queueDeleter) (string, string, error) {
	dispatchErr := dispatch(ctx, action, handlers)
	if dispatchErr != nil {
		slog.Error("queue message dispatch failed", "action", action.Name, "err", dispatchErr)
		if !isPermanentDispatchError(dispatchErr) {
			if raw.ApproximateReceiveCount < maxQueueReceives {
				return workerActionName(action.Name), "failed", dispatchErr
			}
			slog.Error("queue message dropped after max receives", "action", action.Name, "receives", raw.ApproximateReceiveCount, "err", dispatchErr)
			err := deleteProcessedMessage(ctx, deleter, queueURL, raw, action.Name)
			if err != nil {
				return workerActionName(action.Name), "failed", err
			}

			return workerActionName(action.Name), "dropped", nil
		}
	}

	err := deleteProcessedMessage(ctx, deleter, queueURL, raw, action.Name)
	if err != nil {
		return workerActionName(action.Name), "failed", err
	}
	outcome := "processed"
	if dispatchErr != nil {
		outcome = "discarded"
	}

	return workerActionName(action.Name), outcome, nil
}

// firstMetrics returns the optional metrics collector supplied at application wiring.
func firstMetrics(metrics []*Metrics) *Metrics {
	if len(metrics) == 0 {
		return nil
	}

	return metrics[0]
}

// workerActionName maps malformed actions to one bounded metric label.
func workerActionName(action string) string {
	switch action {
	case queue.ActionJungHelp, queue.ActionTopTen, queue.ActionTopDiver,
		queue.ActionAllJung, queue.ActionOffFromWork, queue.ActionEnableAllJung,
		queue.ActionDisableAllJung, queue.ActionSetOffWorkTime, queue.ActionOnOffFromWork,
		queue.ActionSaveMessage:
		return action
	default:
		return "unknown"
	}
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
	return errors.Is(err, ErrPermanentDispatch)
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
func withSetOffInput(handler func(ctx context.Context, input SetOffInput) error, actionName string) actionDispatcher {
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

		return requiredHandler(ctx, SetOffInput{
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
			return permanentDispatchError("missing timeString for %s", actionName)
		}
		_, err = ParseScheduledTime(timeString)
		if err != nil {
			return permanentDispatchError("invalid timeString for %s: %v", actionName, err)
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
		return 0, permanentDispatchError("missing %s for %s", key, action.Name)
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, permanentDispatchError("invalid %s for %s: %v", key, action.Name, err)
	}

	return value, nil
}

// requireHandler returns a configured handler for an action.
func requireHandler[T any](handler T, actionName string) (T, error) {
	value := reflect.ValueOf(handler)
	if !value.IsValid() || value.IsNil() {
		var zero T
		return zero, permanentDispatchError("missing handler for %s", actionName)
	}

	return handler, nil
}

// permanentDispatchError wraps one validation error as non-retryable.
// For example, "missing chatId for topten" becomes a permanent dispatch error.
func permanentDispatchError(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrPermanentDispatch}, args...)...)
}
