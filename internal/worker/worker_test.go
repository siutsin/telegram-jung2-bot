package worker

import (
	"context"
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
		{name: "jung help", action: action(queue.ActionJungHelp), wantCalled: "junghelp"},
		{name: "top ten", action: action(queue.ActionTopTen), wantCalled: "topten"},
		{name: "top diver", action: action(queue.ActionTopDiver), wantCalled: "topdiver"},
		{name: "all jung", action: action(queue.ActionAllJung), wantCalled: "alljung"},
		{name: "off from work", action: action(queue.ActionOffFromWork), wantCalled: "offFromWork"},
		{name: "enable", action: action(queue.ActionEnableAllJung), wantCalled: "enableAllJung"},
		{name: "disable", action: action(queue.ActionDisableAllJung), wantCalled: "disableAllJung"},
		{name: "set off", action: action(queue.ActionSetOffWorkTime), wantCalled: "setOffFromWorkTimeUTC"},
		{name: "on off", action: queue.Action{Name: queue.ActionOnOffFromWork, Attributes: map[string]string{"timeString": "2022-03-04T10:00:00Z"}}, wantCalled: "onOffFromWork"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := make([]string, 0, 1)
			err := Dispatch(context.Background(), test.action, handlers(&calls, nil))

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
	assert.Equal(t, "queue-url", queueWorker.QueueURL)
	assert.Equal(t, "queue-url", queueWorker.Consumer.QueueURL)
	assert.Equal(t, receiver, queueWorker.Consumer.Receiver)
	assert.Equal(t, deleter, queueWorker.Deleter)
	assert.Equal(t, handlers, queueWorker.Handlers)
}

func TestNewPollingWorkerRequiresQueueContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		receiver queue.Receiver
		deleter  Deleter
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
	raw := queue.RawMessage{
		ReceiptHandle: "receipt",
		MessageAttributes: map[string]queue.MessageAttribute{
			"action": mustAttribute(t, `{"StringValue":"topten"}`),
			"chatId": mustAttribute(t, `{"StringValue":"123"}`),
		},
	}
	deleter := &fakeDeleter{}
	receiver := &workerReceiver{response: queue.ReceiveMessageResponse{Messages: []queue.RawMessage{raw}}}
	handlerSet := handlers(nil, nil)
	handlerSet.TopTen = func(ctx context.Context, chatID int64) error {
		cancel()
		return nil
	}

	err := (PollingWorker{
		Consumer: queue.Consumer{QueueURL: "queue-url", Receiver: receiver},
		QueueURL: "queue-url",
		Handlers: handlerSet,
		Deleter:  deleter,
	}).Run(ctx)

	require.NoError(t, err)
	assert.Equal(t, []queue.DeleteMessageRequest{{QueueURL: "queue-url", ReceiptHandle: "receipt"}}, deleter.requests)
}

func TestPollingWorkerContinuesAfterMessageFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	rawMessages := []queue.RawMessage{
		{
			ReceiptHandle: "one",
			MessageAttributes: map[string]queue.MessageAttribute{
				"action": mustAttribute(t, `{"StringValue":"topten"}`),
				"chatId": mustAttribute(t, `{"StringValue":"123"}`),
			},
		},
		{
			ReceiptHandle: "two",
			MessageAttributes: map[string]queue.MessageAttribute{
				"action": mustAttribute(t, `{"StringValue":"topten"}`),
				"chatId": mustAttribute(t, `{"StringValue":"123"}`),
			},
		},
	}
	deleter := &fakeDeleter{}
	receiver := &workerReceiver{response: queue.ReceiveMessageResponse{Messages: rawMessages}}
	handlerSet := handlers(nil, nil)
	handlerSet.TopTen = func(ctx context.Context, chatID int64) error {
		if calls.Add(1) == 1 {
			return errors.New("boom")
		}
		cancel()
		return nil
	}

	err := (PollingWorker{
		Consumer: queue.Consumer{QueueURL: "queue-url", Receiver: receiver},
		QueueURL: "queue-url",
		Handlers: handlerSet,
		Deleter:  deleter,
	}).Run(ctx)

	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())
	require.Len(t, deleter.requests, 1)
	assert.Equal(t, "queue-url", deleter.requests[0].QueueURL)
	assert.Contains(t, []string{"one", "two"}, deleter.requests[0].ReceiptHandle)
}

func TestPollingWorkerRequiresDeleter(t *testing.T) {
	t.Parallel()

	err := (PollingWorker{}).Run(context.Background())

	require.Error(t, err)
	assert.EqualError(t, err, "deleter is required")
}

func TestPollingWorkerReturnsPollError(t *testing.T) {
	t.Parallel()

	err := (PollingWorker{
		Consumer: queue.Consumer{Receiver: &workerReceiver{err: errors.New("boom")}},
		Deleter:  &fakeDeleter{},
	}).Run(context.Background())

	require.Error(t, err)
	assert.EqualError(t, err, "receive SQS messages: boom")
}

func TestPollingWorkerStopsOnCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := (PollingWorker{Deleter: &fakeDeleter{}}).Run(ctx)

	require.NoError(t, err)
}

func TestDispatchPassesSetOffInput(t *testing.T) {
	t.Parallel()

	var input SetOffInput
	handlerSet := handlers(nil, nil)
	handlerSet.SetOffWorkTime = func(ctx context.Context, received SetOffInput) error {
		input = received
		return nil
	}

	err := Dispatch(context.Background(), action(queue.ActionSetOffWorkTime), handlerSet)

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
	handlerSet := handlers(nil, nil)
	handlerSet.JungHelp = func(ctx context.Context, chatID int64, chatTitle string) error {
		helpChatID = chatID
		helpChatTitle = chatTitle
		return nil
	}
	handlerSet.EnableAllJung = func(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
		enableChatID = chatID
		enableChatTitle = chatTitle
		enableUserID = userID
		return nil
	}
	handlerSet.DisableAllJung = func(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
		disableChatID = chatID
		disableChatTitle = chatTitle
		disableUserID = userID
		return nil
	}

	require.NoError(t, Dispatch(context.Background(), action(queue.ActionJungHelp), handlerSet))
	require.NoError(t, Dispatch(context.Background(), action(queue.ActionEnableAllJung), handlerSet))
	require.NoError(t, Dispatch(context.Background(), action(queue.ActionDisableAllJung), handlerSet))

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

	err := Dispatch(context.Background(), action(queue.ActionTopTen), handlers(nil, errors.New("boom")))

	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}

func TestDispatchDropsUnsupportedAction(t *testing.T) {
	t.Parallel()

	err := Dispatch(context.Background(), queue.Action{Name: "nope"}, handlers(nil, nil))

	require.NoError(t, err)
}

func TestDispatchReturnsErrorForMissingHandler(t *testing.T) {
	t.Parallel()

	err := Dispatch(context.Background(), action(queue.ActionTopTen), Handlers{})

	require.Error(t, err)
	assert.EqualError(t, err, "missing handler for topten")
}

func TestProcessMessageDeletesAfterSuccessfulDispatch(t *testing.T) {
	t.Parallel()

	deleter := &fakeDeleter{}
	raw := queue.RawMessage{
		ReceiptHandle: "receipt",
		MessageAttributes: map[string]queue.MessageAttribute{
			"action": mustAttribute(t, `{"StringValue":"topten"}`),
			"chatId": mustAttribute(t, `{"StringValue":"123"}`),
		},
	}

	err := ProcessMessage(context.Background(), "queue-url", raw, handlers(nil, nil), deleter)

	require.NoError(t, err)
	assert.Equal(t, []queue.DeleteMessageRequest{{QueueURL: "queue-url", ReceiptHandle: "receipt"}}, deleter.requests)
}

func TestProcessMessageKeepsMessageAndContinuesOnDispatchFailure(t *testing.T) {
	t.Parallel()

	deleter := &fakeDeleter{}
	raw := queue.RawMessage{MessageAttributes: map[string]queue.MessageAttribute{
		"action": mustAttribute(t, `{"StringValue":"topten"}`),
	}}

	err := ProcessMessage(context.Background(), "queue-url", raw, handlers(nil, errors.New("boom")), deleter)

	require.NoError(t, err)
	assert.Empty(t, deleter.requests)
}

func TestProcessMessageDropsMessageWithoutAction(t *testing.T) {
	t.Parallel()

	err := ProcessMessage(context.Background(), "queue-url", queue.RawMessage{}, handlers(nil, nil), &fakeDeleter{})

	require.NoError(t, err)
}

func TestProcessMessageReturnsDeleteError(t *testing.T) {
	t.Parallel()

	raw := queue.RawMessage{MessageAttributes: map[string]queue.MessageAttribute{
		"action": mustAttribute(t, `{"StringValue":"topten"}`),
	}}

	err := ProcessMessage(context.Background(), "queue-url", raw, handlers(nil, nil), &fakeDeleter{err: errors.New("boom")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete SQS message")
}

func action(name string) queue.Action {
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

func handlers(calls *[]string, err error) Handlers {
	record := func(name string) error {
		if calls != nil {
			*calls = append(*calls, name)
		}
		return err
	}

	return Handlers{
		JungHelp: func(ctx context.Context, chatID int64, chatTitle string) error {
			return record("junghelp")
		},
		TopTen: func(ctx context.Context, chatID int64) error {
			return record("topten")
		},
		TopDiver: func(ctx context.Context, chatID int64) error {
			return record("topdiver")
		},
		AllJung: func(ctx context.Context, chatID int64) error {
			return record("alljung")
		},
		OffFromWork: func(ctx context.Context, chatID int64) error {
			return record("offFromWork")
		},
		EnableAllJung: func(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
			return record("enableAllJung")
		},
		DisableAllJung: func(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
			return record("disableAllJung")
		},
		SetOffWorkTime: func(ctx context.Context, input SetOffInput) error {
			return record("setOffFromWorkTimeUTC")
		},
		OnOffFromWork: func(ctx context.Context, timeString string) error {
			return record("onOffFromWork")
		},
	}
}

func mustAttribute(t *testing.T, raw string) queue.MessageAttribute {
	t.Helper()

	var attribute queue.MessageAttribute
	require.NoError(t, attribute.UnmarshalJSON([]byte(raw)))
	return attribute
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
