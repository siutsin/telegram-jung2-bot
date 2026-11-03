package bot

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// commandsChecker is the Telegram call that reports the account state.
type commandsChecker interface {
	GetMyCommands(ctx context.Context) error
}

// readinessChecker polls Telegram and keeps the readiness flag in line
// with the account state the Bot API returns.
type readinessChecker struct {
	client    commandsChecker
	readiness *atomic.Bool
	stopping  *atomic.Bool
	interval  time.Duration
	timeout   time.Duration

	// ownsNotReady is true only while the current false came from this
	// checker. Only Run's goroutine touches it.
	ownsNotReady bool
}

// NewReadinessChecker builds the Telegram readiness checker. Only a
// frozen account marks readiness false, and the next successful check
// undoes that false. The app sets stopping when teardown begins, and
// from then on the checker stores nothing. For example, a 60 second
// interval with a 10 second timeout checks once a minute and never
// waits longer than 10 seconds for Telegram.
func NewReadinessChecker(client commandsChecker, readiness *atomic.Bool, stopping *atomic.Bool, interval time.Duration, timeout time.Duration) *readinessChecker {
	if stopping == nil {
		stopping = &atomic.Bool{}
	}

	return &readinessChecker{
		client:    client,
		readiness: readiness,
		stopping:  stopping,
		interval:  interval,
		timeout:   timeout,
	}
}

// Run checks once at start, then on every interval tick, until the context
// is cancelled. A failed check never stops the loop and never returns an
// error, because the app tears down every runner that returns one.
func (checker *readinessChecker) Run(ctx context.Context) error {
	checker.check(ctx)

	ticker := time.NewTicker(checker.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			checker.check(ctx)
		}
	}
}

// check makes one account state call and applies the result. Generic
// failures only get logged: readiness tracks the account state, not
// whether Telegram or the network had a bad minute.
func (checker *readinessChecker) check(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, checker.timeout)
	err := checker.client.GetMyCommands(checkCtx)
	cancel()

	switch {
	case err == nil:
		checker.markReady(ctx)
	case IsFrozen(err):
		checker.markNotReady(ctx, err)
	default:
		if ctx.Err() == nil {
			slog.Error("telegram readiness check failed", "err", err)
		}
	}
}

// markReady undoes a false this checker stored. It never overrides the
// app lifecycle: once the app sets stopping for teardown, or the
// context is done, a late success must not put a dying pod back in the
// endpoints.
func (checker *readinessChecker) markReady(ctx context.Context) {
	if !checker.ownsNotReady || checker.stopping.Load() || ctx.Err() != nil {
		return
	}
	checker.ownsNotReady = false
	checker.readiness.Store(true)
	slog.Info("telegram readiness restored")
}

// markNotReady stores the frozen state once and takes ownership of the
// false, so markReady can undo exactly this store later.
func (checker *readinessChecker) markNotReady(ctx context.Context, err error) {
	if checker.ownsNotReady || checker.stopping.Load() || ctx.Err() != nil {
		return
	}
	checker.ownsNotReady = true
	checker.readiness.Store(false)
	slog.Error("telegram account state is frozen; marked not ready", "err", err)
}
