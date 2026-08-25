package bot

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

func TestDispatchRoutesActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		action     queue.Action
		wantCalled string
	}{
		{name: "jung help", action: testAction(queue.ActionJungHelp), wantCalled: "junghelp"},
		{name: "top ten", action: testAction(queue.ActionTopTen), wantCalled: "topten"},
		{name: "top diver", action: testAction(queue.ActionTopDiver), wantCalled: "topdiver"},
		{name: "all jung", action: testAction(queue.ActionAllJung), wantCalled: "alljung"},
		{name: "off from work", action: testAction(queue.ActionOffFromWork), wantCalled: "offFromWork"},
		{name: "enable", action: testAction(queue.ActionEnableAllJung), wantCalled: "enableAllJung"},
		{name: "disable", action: testAction(queue.ActionDisableAllJung), wantCalled: "disableAllJung"},
		{name: "set off", action: testAction(queue.ActionSetOffWorkTime), wantCalled: "setOffFromWorkTimeUTC"},
		{name: "on off", action: queue.Action{Name: queue.ActionOnOffFromWork, Attributes: map[string]string{"timeString": "2022-03-04T10:00:00Z"}}, wantCalled: "onOffFromWork"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := make([]string, 0, 1)
			err := dispatch(context.Background(), test.action, testHandlers(&calls, nil))

			require.NoError(t, err)
			assert.Equal(t, []string{test.wantCalled}, calls)
		})
	}
}

func TestNewPollingWorkerBuildsWorker(t *testing.T) {
	t.Parallel()

	deleter := &fakeDeleter{}
	receiver := &workerReceiver{}
	handlers := Handlers{}
	metrics := NewMetrics(nil)

	queueWorker, err := NewPollingWorker("queue-url", receiver, deleter, handlers, metrics)

	require.NoError(t, err)
	assert.Equal(t, "queue-url", queueWorker.queueURL)
	assert.Equal(t, deleter, queueWorker.deleter)
	assert.Equal(t, handlers, queueWorker.handlers)
	assert.Same(t, metrics, queueWorker.metrics)
	assert.Nil(t, firstMetrics(nil))
}

func TestNewPollingWorkerRequiresQueueContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		receiver queueReceiver
		deleter  queueDeleter
		wantErr  string
	}{
		{name: "missing receiver", deleter: &fakeDeleter{}, wantErr: "queue receiver is required"},
		{name: "missing deleter", receiver: &workerReceiver{}, wantErr: "queue deleter is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewPollingWorker("queue-url", test.receiver, test.deleter, Handlers{})

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
}

func TestPollingWorkerProcessesAndDeletesMessages(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	raw := mustRawMessage(t, `{
		"receiptHandle": "receipt",
		"messageAttributes": {
			"action": {"StringValue": "topten"},
			"chatId": {"StringValue": "123"}
		}
	}`)
	deleter := &fakeDeleter{}
	metrics := NewMetrics(nil)
	receiver := &workerReceiver{response: queue.ReceiveMessageResponse{Messages: []queue.RawMessage{raw}}}
	handlerSet := testHandlers(nil, nil)
	handlerSet.TopTen = func(handlerCtx context.Context, chatID int64) error {
		cancel()
		return nil
	}

	err := (pollingWorker{
		consumer: queue.NewConsumer("queue-url", receiver),
		queueURL: "queue-url",
		handlers: handlerSet,
		deleter:  deleter,
		metrics:  metrics,
	}).Run(ctx)

	require.NoError(t, err)
	assert.Equal(t, []queue.DeleteMessageRequest{{QueueURL: "queue-url", ReceiptHandle: "receipt"}}, deleter.requests)
}

func TestPollingWorkerContinuesAfterHandlerFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	rawMessages := []queue.RawMessage{
		mustRawMessage(t, `{
			"receiptHandle": "one",
			"messageAttributes": {
				"action": {"StringValue": "topten"},
				"chatId": {"StringValue": "123"}
			}
		}`),
	}
	deleter := &fakeDeleter{}
	receiver := &workerReceiver{response: queue.ReceiveMessageResponse{Messages: rawMessages}}
	handlerSet := testHandlers(nil, nil)
	handlerSet.TopTen = func(handlerCtx context.Context, chatID int64) error {
		calls.Add(1)
		if calls.Load() == 2 {
			cancel()
		}

		return errors.New("boom")
	}

	err := (pollingWorker{
		consumer: queue.NewConsumer("queue-url", receiver),
		queueURL: "queue-url",
		handlers: handlerSet,
		deleter:  deleter,
	}).Run(ctx)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, calls.Load(), int32(2))
	assert.Empty(t, deleter.requests)
}

func TestPollingWorkerRequiresDeleter(t *testing.T) {
	t.Parallel()

	err := (pollingWorker{}).Run(context.Background())

	require.Error(t, err)
	assert.EqualError(t, err, "deleter is required")
}

func TestPollingWorkerReturnsPollError(t *testing.T) {
	t.Parallel()

	err := (pollingWorker{
		consumer: queue.NewConsumer("", &workerReceiver{err: errors.New("boom")}),
		deleter:  &fakeDeleter{},
	}).Run(context.Background())

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestPollingWorkerStopsOnCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := (pollingWorker{deleter: &fakeDeleter{}}).Run(ctx)

	require.NoError(t, err)
}

// TestPollingWorkerRecordsQueueWaitWhenEnqueuedAtPresent proves a message
// carrying a valid enqueuedAt attribute records queue wait.
func TestPollingWorkerRecordsQueueWaitWhenEnqueuedAtPresent(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(nil)
	raw := mustRawMessage(t, `{
		"receiptHandle": "receipt",
		"messageAttributes": {
			"action": {"StringValue": "topten"},
			"chatId": {"StringValue": "123"},
			"enqueuedAt": {"StringValue": "2020-01-01T00:00:00Z"}
		}
	}`)

	err := (pollingWorker{
		queueURL: "queue-url",
		handlers: testHandlers(nil, nil),
		deleter:  &fakeDeleter{},
		metrics:  metrics,
	}).processMessage(context.Background(), raw)

	require.NoError(t, err)
	body := scrapeMetrics(t, metrics)
	assert.Contains(t, body, `telegram_jung2_bot_worker_queue_wait_duration_seconds_count{action="topten"} 1`)
}

// TestPollingWorkerSkipsQueueWaitWithoutEnqueuedAt proves a message missing
// the enqueuedAt attribute records no queue wait sample.
func TestPollingWorkerSkipsQueueWaitWithoutEnqueuedAt(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(nil)
	raw := mustRawMessage(t, `{
		"receiptHandle": "receipt",
		"messageAttributes": {
			"action": {"StringValue": "topten"},
			"chatId": {"StringValue": "123"}
		}
	}`)

	err := (pollingWorker{
		queueURL: "queue-url",
		handlers: testHandlers(nil, nil),
		deleter:  &fakeDeleter{},
		metrics:  metrics,
	}).processMessage(context.Background(), raw)

	require.NoError(t, err)
	assert.NotContains(t, scrapeMetrics(t, metrics), "telegram_jung2_bot_worker_queue_wait_duration_seconds")
}

// TestPollingWorkerSkipsQueueWaitWithoutMetrics proves a worker with no
// metrics collector processes messages without panicking.
func TestPollingWorkerSkipsQueueWaitWithoutMetrics(t *testing.T) {
	t.Parallel()

	raw := mustRawMessage(t, `{
		"receiptHandle": "receipt",
		"messageAttributes": {
			"action": {"StringValue": "topten"},
			"chatId": {"StringValue": "123"},
			"enqueuedAt": {"StringValue": "2020-01-01T00:00:00Z"}
		}
	}`)

	err := (pollingWorker{
		queueURL: "queue-url",
		handlers: testHandlers(nil, nil),
		deleter:  &fakeDeleter{},
	}).processMessage(context.Background(), raw)

	require.NoError(t, err)
}

// TestParseEnqueuedAt proves parseEnqueuedAt accepts a valid RFC3339Nano
// timestamp and rejects missing, empty, or malformed values.
func TestParseEnqueuedAt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		attributes map[string]string
		wantOK     bool
	}{
		{name: "missing", attributes: map[string]string{}, wantOK: false},
		{name: "empty", attributes: map[string]string{"enqueuedAt": ""}, wantOK: false},
		{name: "malformed", attributes: map[string]string{"enqueuedAt": "not-a-time"}, wantOK: false},
		{name: "valid", attributes: map[string]string{"enqueuedAt": "2020-01-01T00:00:00Z"}, wantOK: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, ok := parseEnqueuedAt(test.attributes)
			assert.Equal(t, test.wantOK, ok)
		})
	}
}

func TestDispatchPassesSetOffInput(t *testing.T) {
	t.Parallel()

	var input SetOffInput
	handlerSet := testHandlers(nil, nil)
	handlerSet.SetOffWorkTime = func(handlerCtx context.Context, received SetOffInput) error {
		input = received
		return nil
	}

	err := dispatch(context.Background(), testAction(queue.ActionSetOffWorkTime), handlerSet)

	require.NoError(t, err)
	assert.Equal(t, SetOffInput{
		ChatID:    123,
		ChatTitle: "Group",
		UserID:    456,
		OffTime:   "1800",
		Workday:   "MON",
	}, input)
}

func TestDispatchPassesHelpAndAdminFields(t *testing.T) {
	t.Parallel()

	var helpChatID int64
	var helpChatTitle string
	var enableChatID int64
	var enableChatTitle string
	var enableUserID int64
	var disableChatID int64
	var disableChatTitle string
	var disableUserID int64
	handlerSet := testHandlers(nil, nil)
	handlerSet.JungHelp = func(handlerCtx context.Context, chatID int64, chatTitle string) error {
		helpChatID = chatID
		helpChatTitle = chatTitle
		return nil
	}
	handlerSet.EnableAllJung = func(handlerCtx context.Context, chatID int64, chatTitle string, userID int64) error {
		enableChatID = chatID
		enableChatTitle = chatTitle
		enableUserID = userID
		return nil
	}
	handlerSet.DisableAllJung = func(handlerCtx context.Context, chatID int64, chatTitle string, userID int64) error {
		disableChatID = chatID
		disableChatTitle = chatTitle
		disableUserID = userID
		return nil
	}

	require.NoError(t, dispatch(context.Background(), testAction(queue.ActionJungHelp), handlerSet))
	require.NoError(t, dispatch(context.Background(), testAction(queue.ActionEnableAllJung), handlerSet))
	require.NoError(t, dispatch(context.Background(), testAction(queue.ActionDisableAllJung), handlerSet))

	assert.Equal(t, int64(123), helpChatID)
	assert.Equal(t, "Group", helpChatTitle)
	assert.Equal(t, int64(123), enableChatID)
	assert.Equal(t, "Group", enableChatTitle)
	assert.Equal(t, int64(456), enableUserID)
	assert.Equal(t, int64(123), disableChatID)
	assert.Equal(t, "Group", disableChatTitle)
	assert.Equal(t, int64(456), disableUserID)
}

func TestDispatchReturnsHandlerError(t *testing.T) {
	t.Parallel()

	err := dispatch(context.Background(), testAction(queue.ActionTopTen), testHandlers(nil, errors.New("boom")))

	require.Error(t, err)
	require.EqualError(t, err, "boom")
}

func TestDispatchRejectsUnsupportedAction(t *testing.T) {
	t.Parallel()

	err := dispatch(context.Background(), queue.Action{Name: "nope"}, testHandlers(nil, nil))

	require.ErrorIs(t, err, ErrPermanentDispatch)
	assert.Contains(t, err.Error(), "unsupported action nope")
}

func TestDispatchReturnsErrorForMissingHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		action  queue.Action
		wantErr string
	}{
		{
			name:    "chat id",
			action:  testAction(queue.ActionTopTen),
			wantErr: "missing handler for topten",
		},
		{
			name:    "chat id and title",
			action:  testAction(queue.ActionJungHelp),
			wantErr: "missing handler for junghelp",
		},
		{
			name:    "admin fields",
			action:  testAction(queue.ActionEnableAllJung),
			wantErr: "missing handler for enableAllJung",
		},
		{
			name:    "set off input",
			action:  testAction(queue.ActionSetOffWorkTime),
			wantErr: "missing handler for setOffFromWorkTimeUTC",
		},
		{
			name:    "time string",
			action:  queue.Action{Name: queue.ActionOnOffFromWork, Attributes: map[string]string{"timeString": "2026-05-02T12:00:00Z"}},
			wantErr: "missing handler for onOffFromWork",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := dispatch(context.Background(), test.action, Handlers{})

			require.ErrorIs(t, err, ErrPermanentDispatch)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestProcessMessageDeletesAfterSuccessfulDispatch(t *testing.T) {
	t.Parallel()

	deleter := &fakeDeleter{}
	raw := mustRawMessage(t, `{
		"receiptHandle": "receipt",
		"messageAttributes": {
			"action": {"StringValue": "topten"},
			"chatId": {"StringValue": "123"}
		}
	}`)

	_, _, err := processMessageResult(context.Background(), "queue-url", raw, testHandlers(nil, nil), deleter)

	require.NoError(t, err)
	assert.Equal(t, []queue.DeleteMessageRequest{{QueueURL: "queue-url", ReceiptHandle: "receipt"}}, deleter.requests)
}

func TestProcessMessageKeepsMessageAndReturnsDispatchFailure(t *testing.T) {
	t.Parallel()

	deleter := &fakeDeleter{}
	raw := mustRawMessage(t, `{
		"messageAttributes": {
			"action": {"StringValue": "topten"},
			"chatId": {"StringValue": "123"}
		}
	}`)

	_, _, err := processMessageResult(context.Background(), "queue-url", raw, testHandlers(nil, errors.New("boom")), deleter)

	require.Error(t, err)
	require.EqualError(t, err, "boom")
	assert.Empty(t, deleter.requests)
}

func TestProcessMessageDropsMessageWithoutAction(t *testing.T) {
	t.Parallel()

	_, _, err := processMessageResult(context.Background(), "queue-url", queue.RawMessage{}, testHandlers(nil, nil), &fakeDeleter{})

	require.NoError(t, err)
}

func TestProcessMessageDeletesPermanentDispatchErrors(t *testing.T) {
	t.Parallel()

	deleter := &fakeDeleter{}
	raw := mustRawMessage(t, `{
		"receiptHandle": "receipt",
		"messageAttributes": {
			"action": {"StringValue": "topten"}
		}
	}`)

	_, _, err := processMessageResult(context.Background(), "queue-url", raw, testHandlers(nil, nil), deleter)

	require.NoError(t, err)
	assert.Equal(t, []queue.DeleteMessageRequest{{QueueURL: "queue-url", ReceiptHandle: "receipt"}}, deleter.requests)
}

func TestDispatchReturnsMissingAndInvalidAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		action     queue.Action
		wantErr    string
		wantSubstr string
	}{
		{
			name:    "missing chat id",
			action:  queue.Action{Name: queue.ActionTopTen, Attributes: map[string]string{}},
			wantErr: "missing chatId for topten",
		},
		{
			name:       "invalid chat id",
			action:     queue.Action{Name: queue.ActionTopTen, Attributes: map[string]string{"chatId": "bad"}},
			wantSubstr: "invalid chatId for topten",
		},
		{
			name:    "missing user id",
			action:  queue.Action{Name: queue.ActionEnableAllJung, Attributes: map[string]string{"chatId": "123", "chatTitle": "Group"}},
			wantErr: "missing userId for enableAllJung",
		},
		{
			name:       "invalid user id",
			action:     queue.Action{Name: queue.ActionDisableAllJung, Attributes: map[string]string{"chatId": "123", "chatTitle": "Group", "userId": "bad"}},
			wantSubstr: "invalid userId for disableAllJung",
		},
		{
			name:       "invalid chat id for admin action",
			action:     queue.Action{Name: queue.ActionEnableAllJung, Attributes: map[string]string{"chatId": "bad", "chatTitle": "Group", "userId": "456"}},
			wantSubstr: "invalid chatId for enableAllJung",
		},
		{
			name:    "missing chat id for help",
			action:  queue.Action{Name: queue.ActionJungHelp, Attributes: map[string]string{"chatTitle": "Group"}},
			wantErr: "missing chatId for junghelp",
		},
		{
			name:    "missing user id for set off",
			action:  queue.Action{Name: queue.ActionSetOffWorkTime, Attributes: map[string]string{"chatId": "123", "chatTitle": "Group", "offTime": "1800", "workday": "MON"}},
			wantErr: "missing userId for setOffFromWorkTimeUTC",
		},
		{
			name:    "missing chat id for set off",
			action:  queue.Action{Name: queue.ActionSetOffWorkTime, Attributes: map[string]string{"chatTitle": "Group", "userId": "456", "offTime": "1800", "workday": "MON"}},
			wantErr: "missing chatId for setOffFromWorkTimeUTC",
		},
		{
			name:    "missing time string",
			action:  queue.Action{Name: queue.ActionOnOffFromWork, Attributes: map[string]string{}},
			wantErr: "missing timeString for onOffFromWork",
		},
		{
			name:       "invalid time string",
			action:     queue.Action{Name: queue.ActionOnOffFromWork, Attributes: map[string]string{"timeString": "bad"}},
			wantSubstr: "invalid timeString for onOffFromWork",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := dispatch(context.Background(), test.action, testHandlers(nil, nil))

			require.ErrorIs(t, err, ErrPermanentDispatch)
			if test.wantErr != "" {
				assert.Contains(t, err.Error(), test.wantErr)
			}
			if test.wantSubstr != "" {
				assert.Contains(t, err.Error(), test.wantSubstr)
			}
		})
	}
}

func TestIsPermanentDispatchErrorMatchesTypedValidationErrors(t *testing.T) {
	t.Parallel()

	assert.True(t, isPermanentDispatchError(permanentDispatchError("missing chatId for %s", queue.ActionTopTen)))
	assert.True(t, isPermanentDispatchError(permanentDispatchError("invalid chatId for %s: boom", queue.ActionTopTen)))
	assert.True(t, isPermanentDispatchError(permanentDispatchError("missing handler for %s", queue.ActionTopTen)))
	assert.False(t, isPermanentDispatchError(errors.New("invalid UpdateExpression")))
	assert.False(t, isPermanentDispatchError(errors.New("boom")))
	assert.False(t, isPermanentDispatchError(nil))
}

func TestProcessMessageReturnsDeleteErrorAfterPermanentDispatchFailure(t *testing.T) {
	t.Parallel()

	raw := mustRawMessage(t, `{
		"receiptHandle": "receipt",
		"messageAttributes": {
			"action": {"StringValue": "topten"}
		}
	}`)

	_, _, err := processMessageResult(context.Background(), "queue-url", raw, testHandlers(nil, nil), &fakeDeleter{err: errors.New("boom")})

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestProcessMessageReturnsDeleteError(t *testing.T) {
	t.Parallel()

	raw := mustRawMessage(t, `{
		"receiptHandle": "receipt",
		"messageAttributes": {
			"action": {"StringValue": "topten"},
			"chatId": {"StringValue": "123"}
		}
	}`)

	_, _, err := processMessageResult(context.Background(), "queue-url", raw, testHandlers(nil, nil), &fakeDeleter{err: errors.New("boom")})

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func testAction(name string) queue.Action {
	return queue.Action{
		Name: name,
		Attributes: map[string]string{
			"chatId":    "123",
			"chatTitle": "Group",
			"userId":    "456",
			"offTime":   "1800",
			"workday":   "MON",
		},
	}
}

func testHandlers(calls *[]string, err error) Handlers {
	record := func(name string) error {
		if calls != nil {
			*calls = append(*calls, name)
		}
		return err
	}

	return Handlers{
		JungHelp: func(handlerCtx context.Context, chatID int64, chatTitle string) error {
			return record("junghelp")
		},
		TopTen: func(handlerCtx context.Context, chatID int64) error {
			return record("topten")
		},
		TopDiver: func(handlerCtx context.Context, chatID int64) error {
			return record("topdiver")
		},
		AllJung: func(handlerCtx context.Context, chatID int64) error {
			return record("alljung")
		},
		OffFromWork: func(handlerCtx context.Context, chatID int64) error {
			return record("offFromWork")
		},
		EnableAllJung: func(handlerCtx context.Context, chatID int64, chatTitle string, userID int64) error {
			return record("enableAllJung")
		},
		DisableAllJung: func(handlerCtx context.Context, chatID int64, chatTitle string, userID int64) error {
			return record("disableAllJung")
		},
		SetOffWorkTime: func(handlerCtx context.Context, input SetOffInput) error {
			return record("setOffFromWorkTimeUTC")
		},
		OnOffFromWork: func(handlerCtx context.Context, timeString string) error {
			return record("onOffFromWork")
		},
	}
}

func mustRawMessage(t *testing.T, raw string) queue.RawMessage {
	t.Helper()

	var message queue.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &message))
	return message
}

type fakeDeleter struct {
	requests []queue.DeleteMessageRequest
	err      error
}

func (deleter *fakeDeleter) Delete(ctx context.Context, request queue.DeleteMessageRequest) error {
	deleter.requests = append(deleter.requests, request)
	return deleter.err
}

type workerReceiver struct {
	response queue.ReceiveMessageResponse
	err      error
}

func (receiver *workerReceiver) ReceiveMessage(ctx context.Context, request queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error) {
	return receiver.response, receiver.err
}
