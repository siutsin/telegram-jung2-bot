package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Response{StatusCode: 200, Message: "ok"}, Health())
}

func TestNewRoutesHealth(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil)})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "ok", response.Body.String())
}

func TestNewServerBuildsConfiguredHTTPServer(t *testing.T) {
	t.Parallel()

	server, err := NewServer(
		":3000",
		5*time.Second,
		"dev",
		testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil),
	)

	require.NoError(t, err)
	assert.Equal(t, ":3000", server.Addr)
	assert.Equal(t, 5*time.Second, server.ReadTimeout)
	assert.Equal(t, 5*time.Second, server.WriteTimeout)

	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/ping", nil))
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestNewServerRejectsInvalidDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewServer(":3000", 5*time.Second, "dev", Dependencies{})

	require.Error(t, err)
	assert.EqualError(t, err, "validate HTTP dependencies: message store is required")
}

func TestNewRejectsUnsupportedHealthMethod(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil)})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/health", nil))

	assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
}

func TestNewRoutesWebhook(t *testing.T) {
	t.Parallel()

	messages := &fakeMessageStore{}
	chats := &fakeChatStore{}
	enqueuer := &fakeEnqueuer{}
	handler := New(ServerDeps{Dependencies: testDependencies(messages, chats, enqueuer, nil)})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/topTen","entities":[{"type":"bot_command"}]}}`))

	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Len(t, messages.messages, 1)
	assert.Len(t, enqueuer.actions, 1)
}

func TestNewRejectsUnsupportedWebhookMethod(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil)})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/webhook", nil))

	assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
}

func TestNewRejectsOversizedWebhookBody(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{
		Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil),
		MaxBodyBytes: 1,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("{}")))

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "read request body", response.Body.String())
}

func TestNewUsesDefaultWebhookBodyLimit(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil)})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"edited_message":{"text":"ignored"}}`)))

	assert.Equal(t, http.StatusNoContent, response.Code)
}

func TestNewRoutesContractWebhookAndHealthPaths(t *testing.T) {
	t.Parallel()

	messages := &fakeMessageStore{}
	chats := &fakeChatStore{}
	enqueuer := &fakeEnqueuer{}
	handler := New(ServerDeps{Dependencies: testDependencies(messages, chats, enqueuer, nil), Stage: "dev"})

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/ping", nil))
	assert.Equal(t, http.StatusOK, health.Code)
	assert.JSONEq(t, `{"health":"ok"}`, health.Body.String())

	webhook := httptest.NewRecorder()
	handler.ServeHTTP(webhook, httptest.NewRequest(http.MethodPost, "/jung2bot/dev/", strings.NewReader(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"hi"}}`)))
	assert.Equal(t, http.StatusOK, webhook.Code)
	assert.JSONEq(t, `{"statusCode":200}`, webhook.Body.String())
	assert.Len(t, messages.messages, 1)
}

func TestNewContractWebhookRequiresExactStagePath(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil), Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/jung2bot/dev/extra", strings.NewReader(`{}`)))

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestNewContractWebhookRejectsMissingTrailingSlash(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil), Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/jung2bot/dev", strings.NewReader(`{}`)))

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestNewRoutesContractOffFromWork(t *testing.T) {
	t.Parallel()

	enqueuer := &fakeEnqueuer{}
	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, enqueuer, nil), Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onOffFromWork?timeString=2026-05-02T12:00:00Z", nil))

	assert.Equal(t, http.StatusAccepted, response.Code)
	assert.JSONEq(t, `{"onOffFromWork":"ok"}`, response.Body.String())
	require.Len(t, enqueuer.actions, 1)
	assert.Equal(t, queue.ActionOnOffFromWork, enqueuer.actions[0].Name)
	assert.Equal(t, "2026-05-02T12:00:00Z", enqueuer.actions[0].Attributes["timeString"])
}

func TestNewContractOffFromWorkReturnsServerError(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{err: errors.New("boom")}, nil), Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onOffFromWork", nil))

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "Internal Server Error\n", response.Body.String())
}

func TestNewContractOffFromWorkRejectsUnsupportedMethod(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil), Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/jung2bot/dev/onOffFromWork", nil))

	assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
}

func TestNewRoutesContractScaleUp(t *testing.T) {
	t.Parallel()

	dependencies := testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil)
	scaleUpper := &fakeScaleUpper{}
	dependencies.ScaleUpper = scaleUpper
	handler := New(ServerDeps{Dependencies: dependencies, Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onScaleUp", nil))

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"onScaleUp":"ok"}`, response.Body.String())
	assert.True(t, scaleUpper.called)
}

func TestNewContractScaleUpReturnsFailure(t *testing.T) {
	t.Parallel()

	dependencies := testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil)
	dependencies.ScaleUpper = &fakeScaleUpper{err: errors.New("boom")}
	handler := New(ServerDeps{Dependencies: dependencies, Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onScaleUp", nil))

	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.JSONEq(t, `{"onScaleUp":"failed"}`, response.Body.String())
}

func TestNewContractScaleUpRequiresDependency(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil), Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jung2bot/dev/onScaleUp", nil))

	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.JSONEq(t, `{"onScaleUp":"failed"}`, response.Body.String())
}

func TestNewContractScaleUpRejectsUnsupportedMethod(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil), Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/jung2bot/dev/onScaleUp", nil))

	assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
}

func TestHandleWebhookSavesAndEnqueuesCommand(t *testing.T) {
	t.Parallel()

	messages := &fakeMessageStore{}
	chats := &fakeChatStore{}
	enqueuer := &fakeEnqueuer{}
	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"supergroup"},"from":{"id":456},"text":"/topTen","entities":[{"type":"bot_command"}]}}`), testDependencies(messages, chats, enqueuer, nil))

	assert.Equal(t, Response{StatusCode: 200}, response)
	require.Len(t, messages.messages, 1)
	assert.Equal(t, int64(123), messages.messages[0].ChatID)
	assert.Equal(t, "2026-05-02T20:00:00+08:00", message.FormatDateCreated(messages.messages[0].DateCreated))
	require.Len(t, chats.chats, 1)
	assert.Equal(t, int64(123), chats.chats[0].ChatID)
	require.Len(t, enqueuer.actions, 1)
	assert.Equal(t, queue.ActionTopTen, enqueuer.actions[0].Name)
	assert.Equal(t, "123", enqueuer.actions[0].Attributes["chatId"])
}

func TestHandleWebhookEnqueuesMultipleCommandsInContractOrder(t *testing.T) {
	t.Parallel()

	enqueuer := &fakeEnqueuer{}
	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/allJung /topTen /jungHelp","entities":[{"type":"bot_command"}]}}`), testDependencies(&fakeMessageStore{}, &fakeChatStore{}, enqueuer, nil))

	assert.Equal(t, Response{StatusCode: 200}, response)
	require.Len(t, enqueuer.actions, 3)
	assert.Equal(t, queue.ActionJungHelp, enqueuer.actions[0].Name)
	assert.Equal(t, queue.ActionTopTen, enqueuer.actions[1].Name)
	assert.Equal(t, queue.ActionAllJung, enqueuer.actions[2].Name)
}

func TestHandleWebhookInvalidSetOffDoesNotBlockOtherCommands(t *testing.T) {
	t.Parallel()

	messenger := &fakeMessenger{}
	enqueuer := &fakeEnqueuer{}
	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"from":{"id":456},"text":"/setOffFromWorkTimeUTC bad /topTen","entities":[{"type":"bot_command"}]}}`), testDependencies(&fakeMessageStore{}, &fakeChatStore{}, enqueuer, messenger))

	assert.Equal(t, Response{StatusCode: 200}, response)
	require.Len(t, messenger.messages, 1)
	require.Len(t, enqueuer.actions, 1)
	assert.Equal(t, queue.ActionTopTen, enqueuer.actions[0].Name)
}

func TestHandleWebhookIgnoresUnsupportedUpdate(t *testing.T) {
	t.Parallel()

	response := HandleWebhook(context.Background(), []byte(`{"edited_message":{"text":"ignored"}}`), testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil))

	assert.Equal(t, Response{StatusCode: 204, Message: "edited_message or non-group"}, response)
}

func TestHandleWebhookIgnoresNonGroup(t *testing.T) {
	t.Parallel()

	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"type":"private"},"text":"hi"}}`), testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil))

	assert.Equal(t, Response{StatusCode: 204, Message: "edited_message or non-group"}, response)
}

func TestHandleWebhookReturnsDecodeError(t *testing.T) {
	t.Parallel()

	response := HandleWebhook(context.Background(), []byte(`{bad json`), testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil))

	assert.Equal(t, Response{StatusCode: 500, Message: "decode Telegram update"}, response)
}

func TestHandleWebhookSavesPlainMessageWithoutEnqueue(t *testing.T) {
	t.Parallel()

	messages := &fakeMessageStore{}
	chats := &fakeChatStore{}
	enqueuer := &fakeEnqueuer{}
	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"hello"}}`), testDependencies(messages, chats, enqueuer, nil))

	assert.Equal(t, Response{StatusCode: 200}, response)
	assert.Len(t, messages.messages, 1)
	assert.Empty(t, enqueuer.actions)
}

func TestHandleWebhookIgnoresSlashTextWithoutBotCommandEntity(t *testing.T) {
	t.Parallel()

	messages := &fakeMessageStore{}
	chats := &fakeChatStore{}
	enqueuer := &fakeEnqueuer{}
	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/topTen"}}`), testDependencies(messages, chats, enqueuer, nil))

	assert.Equal(t, Response{StatusCode: 200}, response)
	assert.Len(t, messages.messages, 1)
	assert.Empty(t, enqueuer.actions)
}

func TestHandleWebhookRequiresFirstEntityToBeBotCommand(t *testing.T) {
	t.Parallel()

	enqueuer := &fakeEnqueuer{}
	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/topTen","entities":[{"type":"mention"},{"type":"bot_command"}]}}`), testDependencies(&fakeMessageStore{}, &fakeChatStore{}, enqueuer, nil))

	assert.Equal(t, Response{StatusCode: 200}, response)
	assert.Empty(t, enqueuer.actions)
}

func TestHandleWebhookSendsInvalidSetOffReply(t *testing.T) {
	t.Parallel()

	messenger := &fakeMessenger{}
	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"from":{"id":456},"text":"/setOffFromWorkTimeUTC bad","entities":[{"type":"bot_command"}]}}`), testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, messenger))

	assert.Equal(t, Response{StatusCode: 200}, response)
	require.Len(t, messenger.messages, 1)
	assert.Contains(t, messenger.messages[0], "Error: Invalid format for setOffFromWorkTimeUTC")
}

func TestHandleWebhookReturnsReplyErrorWhenMessengerIsMissing(t *testing.T) {
	t.Parallel()

	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/setOffFromWorkTimeUTC bad","entities":[{"type":"bot_command"}]}}`), Dependencies{
		Messages: &fakeMessageStore{},
		Chats:    &fakeChatStore{},
		Enqueuer: &fakeEnqueuer{},
		Now: func() time.Time {
			return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		},
	})

	assert.Equal(t, Response{StatusCode: 500, Message: "reply invalid command"}, response)
}

func TestHandleWebhookReturnsSaveMessageError(t *testing.T) {
	t.Parallel()

	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"hello"}}`), testDependencies(&fakeMessageStore{saveErr: errors.New("boom")}, &fakeChatStore{}, &fakeEnqueuer{}, nil))

	assert.Equal(t, Response{StatusCode: 500, Message: "save message"}, response)
}

func TestHandleWebhookReturnsSaveChatError(t *testing.T) {
	t.Parallel()

	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"hello"}}`), testDependencies(&fakeMessageStore{}, &fakeChatStore{saveErr: errors.New("boom")}, &fakeEnqueuer{}, nil))

	assert.Equal(t, Response{StatusCode: 500, Message: "save chat"}, response)
}

func TestHandleWebhookReturnsEnqueueError(t *testing.T) {
	t.Parallel()

	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"/jungHelp","entities":[{"type":"bot_command"}]}}`), testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{err: errors.New("boom")}, nil))

	assert.Equal(t, Response{StatusCode: 500, Message: "enqueue command"}, response)
}

func TestNewContractWebhookSuppressesInternalErrorMessage(t *testing.T) {
	t.Parallel()

	handler := New(ServerDeps{Dependencies: testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil), Stage: "dev"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/jung2bot/dev/", strings.NewReader(`{bad json`)))

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.JSONEq(t, `{"statusCode":500}`, response.Body.String())
}

func TestHandleWebhookDefaultsTime(t *testing.T) {
	t.Parallel()

	messages := &fakeMessageStore{}
	dependencies := testDependencies(messages, &fakeChatStore{}, &fakeEnqueuer{}, nil)
	dependencies.Now = nil
	response := HandleWebhook(context.Background(), []byte(`{"message":{"chat":{"id":123,"title":"Group","type":"group"},"text":"hello"}}`), dependencies)

	assert.Equal(t, 200, response.StatusCode)
	assert.False(t, messages.messages[0].DateCreated.IsZero())
}

func TestValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, Validate(testDependencies(&fakeMessageStore{}, &fakeChatStore{}, &fakeEnqueuer{}, nil)))
	require.EqualError(t, Validate(Dependencies{}), "message store is required")
	require.EqualError(t, Validate(Dependencies{Messages: &fakeMessageStore{}}), "chat store is required")
	require.EqualError(t, Validate(Dependencies{Messages: &fakeMessageStore{}, Chats: &fakeChatStore{}}), "enqueuer is required")
	require.EqualError(t, Validate(Dependencies{Messages: &fakeMessageStore{}, Chats: &fakeChatStore{}, Enqueuer: &fakeEnqueuer{}}), "messenger is required")
}

func testDependencies(messages MessageSaver, chats ChatSaver, enqueuer Enqueuer, messenger Messenger) Dependencies {
	if messenger == nil {
		messenger = &fakeMessenger{}
	}

	return Dependencies{
		Messages:  messages,
		Chats:     chats,
		Enqueuer:  enqueuer,
		Messenger: messenger,
		Now: func() time.Time {
			return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		},
	}
}

type fakeMessageStore struct {
	messages []message.Message
	saveErr  error
}

func (store *fakeMessageStore) Save(ctx context.Context, tableName string, row message.Message) error {
	store.messages = append(store.messages, row)
	return store.saveErr
}

type fakeChatStore struct {
	chats   []chat.ChatSetting
	saveErr error
}

func (store *fakeChatStore) Save(ctx context.Context, tableName string, settings chat.ChatSetting) error {
	store.chats = append(store.chats, settings)
	return store.saveErr
}

type fakeEnqueuer struct {
	actions []queue.Action
	err     error
}

func (enqueuer *fakeEnqueuer) Enqueue(ctx context.Context, action queue.Action) error {
	enqueuer.actions = append(enqueuer.actions, action)
	return enqueuer.err
}

type fakeMessenger struct {
	err      error
	messages []string
}

func (messenger *fakeMessenger) SendMessage(ctx context.Context, chatID int64, text string) error {
	messenger.messages = append(messenger.messages, text)
	return messenger.err
}

type fakeScaleUpper struct {
	called bool
	err    error
}

func (scaleUpper *fakeScaleUpper) ScaleUp(ctx context.Context) error {
	scaleUpper.called = true
	return scaleUpper.err
}
