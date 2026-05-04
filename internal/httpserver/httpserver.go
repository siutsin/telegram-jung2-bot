// Package httpserver owns transport-independent webhook handling.
package httpserver

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=httpserver.go -destination=../mock/httpserver_mock.go -package=mock -mock_names messageSaver=MockMessageSaver,chatSaver=MockChatSaver,enqueuer=MockEnqueuer,messenger=MockMessenger,scaleUpper=MockScaleUpper"

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

type response struct {
	StatusCode int
	Message    string
}

type messageSaver interface {
	Save(ctx context.Context, tableName string, row message.Message) error
}

type chatSaver interface {
	Save(ctx context.Context, tableName string, settings chat.ChatSetting) error
}

type enqueuer interface {
	Enqueue(ctx context.Context, action queue.Action) error
}

type messenger interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

type scaleUpper interface {
	ScaleUp(ctx context.Context) error
}

type Dependencies struct {
	ChatTable    string
	MessageTable string
	Messages     messageSaver
	Chats        chatSaver
	Enqueuer     enqueuer
	Messenger    messenger
	ScaleUpper   scaleUpper
	Now          func() time.Time
}

type serverDeps struct {
	Dependencies
	MaxBodyBytes int64
	Stage        string
}

// NewServer builds the production HTTP server from validated runtime values.
func NewServer(address string, timeout time.Duration, stage string, dependencies Dependencies) (*http.Server, error) {
	err := validate(dependencies)
	if err != nil {
		return nil, fmt.Errorf("validate HTTP dependencies: %w", err)
	}

	return &http.Server{
		Addr: address,
		Handler: newHandler(serverDeps{
			Dependencies: dependencies,
			Stage:        stage,
		}),
		ReadHeaderTimeout: timeout,
		ReadTimeout:       timeout,
		WriteTimeout:      timeout,
		IdleTimeout:       timeout,
	}, nil
}

// Health returns the health check response.
func health() response {
	return response{StatusCode: 200, Message: "ok"}
}
