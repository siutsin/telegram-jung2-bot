// Package httpserver owns transport-independent webhook handling.
package httpserver

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -destination=../mock/httpserver/httpserver_mock.go -package=httpservermock -mock_names enqueuer=MockEnqueuer,messenger=MockMessenger,scaleUpper=MockScaleUpper . enqueuer,messenger,scaleUpper"

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

type response struct {
	statusCode int
	message    string
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

// metricsRecorder instruments fixed HTTP routes and records webhook outcomes.
type metricsRecorder interface {
	HTTPHandler(route string, handler http.Handler) http.Handler
	RecordWebhookCommand(action string)
	RecordWebhookUpdate(outcome string)
}

type Dependencies struct {
	ChatTable            string
	MessageTable         string
	Enqueuer             enqueuer
	MessageEnqueuer      enqueuer
	Messenger            messenger
	ScaleUpper           scaleUpper
	Now                  func() time.Time
	WebhookSecretToken   string
	SchedulerSecretToken string
	Readiness            *atomic.Bool
	Metrics              metricsRecorder
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
