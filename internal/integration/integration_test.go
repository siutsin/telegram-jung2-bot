package integration

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/app"
	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/command"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
	"github.com/siutsin/telegram-jung2-bot/internal/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/statistics"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

func TestWebhookIntakeSlice(t *testing.T) {
	t.Parallel()

	store := &sliceStore{}
	enqueuer := &sliceEnqueuer{}
	response := httpserver.HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"supergroup"},"from":{"id":456,"first_name":"Ada"},"text":"/topTen","entities":[{"type":"bot_command"}]}}`), httpserver.Dependencies{
		MessageTable: "messages",
		ChatTable:    "chats",
		Store:        store,
		Enqueuer:     enqueuer,
		Now:          fixedNow,
	})

	assert.Equal(t, httpserver.Response{StatusCode: 200}, response)
	messages := store.messageUpdates()
	require.Len(t, messages, 1)
	assert.Equal(t, map[string]any{"chatId": int64(123), "dateCreated": "2026-05-02T20:00:00+08:00"}, messages[0].Key)
	actions := enqueuer.enqueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, queue.ActionTopTen, actions[0].Name)
	assert.Equal(t, "123", actions[0].Attributes["chatId"])
}

func TestCommandExecutionSlice(t *testing.T) {
	t.Parallel()

	rows := []message.Message{
		{ChatID: 123, ChatTitle: "Group", UserID: 1, FirstName: "Ada", DateCreated: fixedNow().Add(-time.Hour)},
		{ChatID: 123, ChatTitle: "Group", UserID: 1, FirstName: "Ada", DateCreated: fixedNow().Add(-2 * time.Hour)},
		{ChatID: 123, ChatTitle: "Group", UserID: 2, FirstName: "Grace", DateCreated: fixedNow().Add(-3 * time.Hour)},
	}
	action, err := command.ActionFor(command.Command{Name: command.TopTen}, command.ChatContext{ChatID: 123, ChatTitle: "Group"})
	require.NoError(t, err)

	rendered := statistics.Render(statistics.Report{
		Rows: rows,
		Options: statistics.Options{
			Limit: 10,
			Now:   fixedNow(),
		},
	})

	assert.Equal(t, queue.ActionTopTen, action.Name)
	assert.Contains(t, rendered, "圍爐區: Group")
	assert.Contains(t, rendered, "1. Ada  66.67%")
	assert.Contains(t, rendered, "2. Grace  33.33%")
}

func TestQueueExecutionSlice(t *testing.T) {
	t.Parallel()

	raw := queue.RawMessage{
		ReceiptHandle: "receipt",
		MessageAttributes: map[string]queue.MessageAttribute{
			"action":    mustAttribute(t, `{"StringValue":"enableAllJung"}`),
			"chatId":    mustAttribute(t, `{"StringValue":"123"}`),
			"chatTitle": mustAttribute(t, `{"StringValue":"Group"}`),
			"userId":    mustAttribute(t, `{"StringValue":"456"}`),
		},
	}
	deleter := &sliceDeleter{}
	var gotChatID int64
	var gotChatTitle string
	var gotUserID int64

	err := worker.ProcessMessage(context.Background(), "queue-url", raw, worker.Handlers{
		EnableAllJung: func(ctx context.Context, chatID int64, chatTitle string, userID int64) error {
			gotChatID = chatID
			gotChatTitle = chatTitle
			gotUserID = userID
			return nil
		},
	}, deleter)

	require.NoError(t, err)
	assert.Equal(t, int64(123), gotChatID)
	assert.Equal(t, "Group", gotChatTitle)
	assert.Equal(t, int64(456), gotUserID)
	assert.Equal(t, []queue.DeleteMessageRequest{{QueueURL: "queue-url", ReceiptHandle: "receipt"}}, deleter.requests)
}

func TestSettingsManagementSlice(t *testing.T) {
	t.Parallel()

	change, err := schedule.SetOffFromWorkTimeUTC("chats", 123, "Group", true, "1800", "MON,WED")

	require.NoError(t, err)
	require.True(t, change.Allowed)
	assert.Equal(t, chat.BuildOffWorkUpdate("chats", 123, "1800", workday.Workdays(workday.Mon|workday.Wed)), change.Update)
	assert.Contains(t, change.Reply, "Updated setOffFromWorkTime in UTC: 1800 MON,WED")
}

func TestScheduledReportsSlice(t *testing.T) {
	t.Parallel()

	enqueuer := &sliceEnqueuer{}
	service := schedule.Service{
		Chats: &sliceChats{rows: []chat.Settings{
			{ChatID: 123},
			{ChatID: 456, OffTime: "1800", HasOffTime: true, Workday: workday.Workdays(workday.Fri), HasWorkday: true},
		}},
		Enqueuer: enqueuer,
	}

	err := service.HandleDueReport(context.Background(), time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	assert.Equal(t, []queue.Action{schedule.BuildOffFromWorkAction(123)}, enqueuer.enqueuedActions())
}

func TestApplicationWiringSlice(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	factory := &sliceFactory{httpServer: newSliceHTTPServer(), queueWorker: &sliceQueueWorker{}}
	application, err := app.New(context.Background(), config.Config{ShutdownTimeout: time.Millisecond}, app.Options{Factory: factory})
	require.NoError(t, err)
	done := make(chan error, 1)

	go func() {
		done <- application.Run(ctx)
	}()

	<-factory.httpServer.started
	cancel()

	require.NoError(t, <-done)
	require.Eventually(t, factory.httpServer.shutdownCalled.Load, time.Second, time.Millisecond)
	require.Eventually(t, factory.queueWorker.cancelled.Load, time.Second, time.Millisecond)
}

func TestApplicationWiringSliceReturnsDependencyErrors(t *testing.T) {
	t.Parallel()

	application, err := app.New(context.Background(), config.Config{}, app.Options{Factory: &sliceFactory{err: errors.New("boom")}})
	require.NoError(t, err)
	err = application.Run(context.Background())

	require.Error(t, err)
	assert.EqualError(t, err, "create HTTP server: boom")
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
}

type sliceStore struct {
	mu       sync.Mutex
	messages []message.UpdateExpression
	chats    []chat.UpdateExpression
}

func (store *sliceStore) SaveMessage(ctx context.Context, request message.UpdateExpression) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.messages = append(store.messages, request)
	return nil
}

func (store *sliceStore) SaveChat(ctx context.Context, request chat.UpdateExpression) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.chats = append(store.chats, request)
	return nil
}

func (store *sliceStore) messageUpdates() []message.UpdateExpression {
	store.mu.Lock()
	defer store.mu.Unlock()

	return append([]message.UpdateExpression(nil), store.messages...)
}

type sliceEnqueuer struct {
	mu      sync.Mutex
	actions []queue.Action
}

func (enqueuer *sliceEnqueuer) Enqueue(ctx context.Context, action queue.Action) error {
	enqueuer.mu.Lock()
	defer enqueuer.mu.Unlock()
	enqueuer.actions = append(enqueuer.actions, action)
	return nil
}

func (enqueuer *sliceEnqueuer) enqueuedActions() []queue.Action {
	enqueuer.mu.Lock()
	defer enqueuer.mu.Unlock()

	return append([]queue.Action(nil), enqueuer.actions...)
}

type sliceChats struct {
	rows []chat.Settings
}

func (chats *sliceChats) ListEnabled(ctx context.Context) ([]chat.Settings, error) {
	return chats.rows, nil
}

type sliceFactory struct {
	httpServer  *sliceHTTPServer
	queueWorker *sliceQueueWorker
	err         error
}

func (factory *sliceFactory) NewHTTPServer(config config.Config) (app.HTTPServer, error) {
	if factory.err != nil {
		return nil, factory.err
	}
	return factory.httpServer, nil
}

func (factory *sliceFactory) NewQueueWorker(config config.Config) (app.QueueWorker, error) {
	return factory.queueWorker, nil
}

type sliceHTTPServer struct {
	started        chan struct{}
	stopped        chan struct{}
	shutdownCalled atomic.Bool
}

func newSliceHTTPServer() *sliceHTTPServer {
	return &sliceHTTPServer{
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (server *sliceHTTPServer) ListenAndServe() error {
	close(server.started)
	<-server.stopped
	return nil
}

func (server *sliceHTTPServer) Shutdown(ctx context.Context) error {
	server.shutdownCalled.Store(true)
	close(server.stopped)
	return nil
}

type sliceQueueWorker struct {
	cancelled atomic.Bool
}

func (worker *sliceQueueWorker) Run(ctx context.Context) error {
	<-ctx.Done()
	worker.cancelled.Store(true)
	return nil
}

func mustAttribute(t *testing.T, raw string) queue.MessageAttribute {
	t.Helper()

	var attribute queue.MessageAttribute
	require.NoError(t, attribute.UnmarshalJSON([]byte(raw)))
	return attribute
}

type sliceDeleter struct {
	requests []queue.DeleteMessageRequest
}

func (deleter *sliceDeleter) Delete(ctx context.Context, request queue.DeleteMessageRequest) error {
	deleter.requests = append(deleter.requests, request)
	return nil
}
