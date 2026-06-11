package worker

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

	queueWorker, err := NewPollingWorker("queue-url", receiver, deleter, handlers)

	require.NoError(t, err)
	assert.Equal(t, "queue-url", queueWorker.queueURL)
	assert.Equal(t, deleter, queueWorker.deleter)
	assert.Equal(t, handlers, queueWorker.handlers)
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
	}).Run(ctx)

	require.NoError(t, err)
	assert.Equal(t, []queue.DeleteMessageRequest{{QueueURL: "queue-url", ReceiptHandle: "receipt"}}, deleter.requests)
}

func TestPollingWorkerReturnsMessageFailure(t *testing.T) {
	t.Parallel()

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
		return errors.New("boom")
	}

	err := (pollingWorker{
		consumer: queue.NewConsumer("queue-url", receiver),
		queueURL: "queue-url",
		handlers: handlerSet,
		deleter:  deleter,
	}).Run(context.Background())

	require.Error(t, err)
	require.EqualError(t, err, "boom")
	assert.Equal(t, int32(1), calls.Load())
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

func TestDispatchDropsUnsupportedAction(t *testing.T) {
	t.Parallel()

	err := dispatch(context.Background(), queue.Action{Name: "nope"}, testHandlers(nil, nil))

	require.NoError(t, err)
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

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
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

	err := processMessage(context.Background(), "queue-url", raw, testHandlers(nil, nil), deleter)

	require.NoError(t, err)
	assert.Equal(t, []queue.DeleteMessageRequest{{QueueURL: "queue-url", ReceiptHandle: "receipt"}}, deleter.requests)
}

func TestProcessMessageKeepsMessageAndReturnsDispatchFailure(t *testing.T) {
	t.Parallel()

	deleter := &fakeDeleter{}
	raw := mustRawMessage(t, `{"messageAttributes":{"action":{"StringValue":"topten"}}}`)

	err := processMessage(context.Background(), "queue-url", raw, testHandlers(nil, errors.New("boom")), deleter)

	require.Error(t, err)
	require.EqualError(t, err, "boom")
	assert.Empty(t, deleter.requests)
}

func TestProcessMessageDropsMessageWithoutAction(t *testing.T) {
	t.Parallel()

	err := processMessage(context.Background(), "queue-url", queue.RawMessage{}, testHandlers(nil, nil), &fakeDeleter{})

	require.NoError(t, err)
}

func TestProcessMessageReturnsDeleteError(t *testing.T) {
	t.Parallel()

	raw := mustRawMessage(t, `{"messageAttributes":{"action":{"StringValue":"topten"}}}`)

	err := processMessage(context.Background(), "queue-url", raw, testHandlers(nil, nil), &fakeDeleter{err: errors.New("boom")})

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
