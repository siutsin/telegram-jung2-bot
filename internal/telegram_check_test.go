package bot

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errFrozenAccount = &APIError{StatusCode: http.StatusBadRequest, Description: "FROZEN_METHOD_INVALID"}

// fakeCommandsChecker scripts GetMyCommands results in call order. Calls
// past the end of the script succeed.
type fakeCommandsChecker struct {
	mutex         sync.Mutex
	results       []error
	calls         int
	waitForCancel bool
}

func (fake *fakeCommandsChecker) GetMyCommands(ctx context.Context) error {
	if fake.waitForCancel {
		<-ctx.Done()
	}

	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	index := fake.calls
	fake.calls++
	if index >= len(fake.results) {
		return nil
	}

	return fake.results[index]
}

func (fake *fakeCommandsChecker) callCount() int {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()

	return fake.calls
}

func TestReadinessCheckerMarksNotReadyWhenFrozen(t *testing.T) {
	readiness := &atomic.Bool{}
	readiness.Store(true)
	fake := &fakeCommandsChecker{results: []error{errFrozenAccount, errFrozenAccount}}
	checker := NewReadinessChecker(fake, readiness, &atomic.Bool{}, time.Second, time.Second)

	checker.check(context.Background())
	assert.False(t, readiness.Load())

	// A second frozen result changes nothing: the false is already owned.
	checker.check(context.Background())
	assert.False(t, readiness.Load())
	assert.Equal(t, 2, fake.callCount())
}

func TestReadinessCheckerRestoresReadinessAfterFrozen(t *testing.T) {
	readiness := &atomic.Bool{}
	readiness.Store(true)
	fake := &fakeCommandsChecker{results: []error{errFrozenAccount, nil, nil}}
	checker := NewReadinessChecker(fake, readiness, &atomic.Bool{}, time.Second, time.Second)

	checker.check(context.Background())
	require.False(t, readiness.Load())

	checker.check(context.Background())
	assert.True(t, readiness.Load())

	// Later successes are no-ops once the owned false has been undone.
	checker.check(context.Background())
	assert.True(t, readiness.Load())
}

// TestReadinessCheckerLeavesLifecycleStateAlone proves a successful check
// never overrides a false the app lifecycle stored: the checker only
// undoes its own false.
func TestReadinessCheckerLeavesLifecycleStateAlone(t *testing.T) {
	readiness := &atomic.Bool{}
	fake := &fakeCommandsChecker{results: []error{nil}}
	checker := NewReadinessChecker(fake, readiness, &atomic.Bool{}, time.Second, time.Second)

	checker.check(context.Background())

	assert.False(t, readiness.Load())
}

func TestReadinessCheckerIgnoresGenericErrors(t *testing.T) {
	readiness := &atomic.Bool{}
	readiness.Store(true)
	fake := &fakeCommandsChecker{results: []error{errors.New("boom")}}
	checker := NewReadinessChecker(fake, readiness, &atomic.Bool{}, time.Second, time.Second)

	checker.check(context.Background())

	assert.True(t, readiness.Load())
}

// TestReadinessCheckerRunChecksUntilCancelled runs the checker loop in a
// goroutine: the first check runs at once, ticks keep running, and
// cancellation ends the loop without an error.
func TestReadinessCheckerRunChecksUntilCancelled(t *testing.T) {
	readiness := &atomic.Bool{}
	readiness.Store(true)
	fake := &fakeCommandsChecker{results: []error{errFrozenAccount, nil}}
	checker := NewReadinessChecker(fake, readiness, &atomic.Bool{}, 5*time.Millisecond, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- checker.Run(ctx)
	}()

	require.Eventually(t, func() bool {
		return fake.callCount() >= 2 && readiness.Load()
	}, 5*time.Second, time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	assert.True(t, readiness.Load())
}

func TestReadinessCheckerRunReturnsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeCommandsChecker{}
	checker := NewReadinessChecker(fake, &atomic.Bool{}, nil, time.Second, time.Second)

	require.NoError(t, checker.Run(ctx))
	assert.Equal(t, 1, fake.callCount())
}

// TestReadinessCheckerStopsStoringWhenAppStops proves the app lifecycle
// wins the drain window. On a component failure app.go sets stopping
// before its final readiness store, but the checker keeps a live
// context until the drain ends, so only the stopping flag can hold a
// late store back.
func TestReadinessCheckerStopsStoringWhenAppStops(t *testing.T) {
	t.Run("success does not undo the owned false", func(t *testing.T) {
		readiness := &atomic.Bool{}
		readiness.Store(true)
		stopping := &atomic.Bool{}
		fake := &fakeCommandsChecker{results: []error{errFrozenAccount, nil}}
		checker := NewReadinessChecker(fake, readiness, stopping, time.Second, time.Second)
		checker.check(context.Background())
		require.False(t, readiness.Load())

		stopping.Store(true)
		checker.check(context.Background())

		assert.False(t, readiness.Load())
	})

	t.Run("frozen result does not store", func(t *testing.T) {
		readiness := &atomic.Bool{}
		readiness.Store(true)
		stopping := &atomic.Bool{}
		stopping.Store(true)
		fake := &fakeCommandsChecker{results: []error{errFrozenAccount}}
		checker := NewReadinessChecker(fake, readiness, stopping, time.Second, time.Second)

		checker.check(context.Background())

		assert.True(t, readiness.Load())
	})
}

// TestReadinessCheckerIgnoresLateResultsAfterCancel proves a check that
// finishes after shutdown began cannot store readiness in either
// direction.
func TestReadinessCheckerIgnoresLateResultsAfterCancel(t *testing.T) {
	t.Run("late success keeps the frozen false", func(t *testing.T) {
		readiness := &atomic.Bool{}
		readiness.Store(true)
		fake := &fakeCommandsChecker{results: []error{errFrozenAccount, nil}}
		checker := NewReadinessChecker(fake, readiness, &atomic.Bool{}, time.Second, time.Second)
		checker.check(context.Background())
		require.False(t, readiness.Load())

		fake.waitForCancel = true
		assertCancelledCheckIgnored(t, checker)
		assert.False(t, readiness.Load())
	})

	t.Run("late frozen result keeps readiness", func(t *testing.T) {
		readiness := &atomic.Bool{}
		readiness.Store(true)
		fake := &fakeCommandsChecker{results: []error{errFrozenAccount}, waitForCancel: true}
		checker := NewReadinessChecker(fake, readiness, &atomic.Bool{}, time.Second, time.Second)

		assertCancelledCheckIgnored(t, checker)
		assert.True(t, readiness.Load())
	})

	t.Run("late generic error keeps readiness", func(t *testing.T) {
		readiness := &atomic.Bool{}
		readiness.Store(true)
		fake := &fakeCommandsChecker{results: []error{errors.New("boom")}, waitForCancel: true}
		checker := NewReadinessChecker(fake, readiness, &atomic.Bool{}, time.Second, time.Second)

		assertCancelledCheckIgnored(t, checker)
		assert.True(t, readiness.Load())
	})
}

// assertCancelledCheckIgnored runs one check in a goroutine and cancels
// its context while the fake Telegram call is still blocked.
func assertCancelledCheckIgnored(t *testing.T, checker *readinessChecker) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		checker.check(ctx)
	}()
	cancel()
	<-done
}
