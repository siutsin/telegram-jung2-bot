// Package httpserver owns transport-independent webhook handling.
package httpserver

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=httpserver.go -destination=../mock/httpserver/httpserver_mock.go -package=httpservermock -mock_names messageSaver=MockMessageSaver,chatSaver=MockChatSaver,enqueuer=MockEnqueuer,messenger=MockMessenger,scaleUpper=MockScaleUpper"

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	bot "github.com/siutsin/telegram-jung2-bot/internal"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

type response struct {
	statusCode int
	message    string
}

type messageSaver interface {
	Save(ctx context.Context, tableName string, row bot.StoredMessage) error
}

type chatSaver interface {
	Save(ctx context.Context, tableName string, settings bot.ChatSetting) error
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
	ChatTable            string
	MessageTable         string
	Messages             messageSaver
	Chats                chatSaver
	Enqueuer             enqueuer
	Messenger            messenger
	ScaleUpper           scaleUpper
	Now                  func() time.Time
	WebhookSecretToken   string
	SchedulerSecretToken string
	Readiness            *atomic.Bool
}

type serverDeps struct {
	Dependencies
	maxBodyBytes int64
	stage        string
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
			stage:        stage,
		}),
		ReadHeaderTimeout: timeout,
		ReadTimeout:       timeout,
		WriteTimeout:      timeout,
		IdleTimeout:       timeout,
	}, nil
}

// health returns the readiness response.
// For example, a false readiness value returns 503 while true returns 200.
func health(readiness *atomic.Bool) response {
	if readiness == nil || !readiness.Load() {
		return response{statusCode: http.StatusServiceUnavailable, message: "not ready"}
	}

	return response{statusCode: 200, message: "ok"}
}
