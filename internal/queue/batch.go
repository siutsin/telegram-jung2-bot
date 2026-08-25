package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	maxBatchSize        = 10
	defaultFlushTimeout = 15 * time.Second
	shutdownFlushLimit  = 5 * time.Second
)

// batchProducer buffers actions until a full batch or the flush timer fires.
type batchProducer struct {
	queueURL     string
	sender       batchMessageSender
	flushTimeout time.Duration

	mutex   sync.Mutex
	actions []Action
	wake    chan struct{}
}

// NewBatchProducer builds a producer that sends at most ten actions per SQS
// request. For example, a zero flush timeout waits 15 seconds.
func NewBatchProducer(queueURL string, sender batchMessageSender, flushTimeout time.Duration) *batchProducer {
	if flushTimeout <= 0 {
		flushTimeout = defaultFlushTimeout
	}

	return &batchProducer{
		queueURL:     queueURL,
		sender:       sender,
		flushTimeout: flushTimeout,
		wake:         make(chan struct{}, 1),
	}
}

// Enqueue accepts an action without waiting for an SQS request.
func (producer *batchProducer) Enqueue(_ context.Context, action Action) error {
	if producer == nil || producer.sender == nil {
		return fmt.Errorf("queue batch sender is required")
	}

	producer.mutex.Lock()
	producer.actions = append(producer.actions, withEnqueuedAt(action))
	producer.mutex.Unlock()
	producer.notify()

	return nil
}

// Run flushes pending actions until the context is cancelled.
func (producer *batchProducer) Run(ctx context.Context) error {
	if producer == nil || producer.sender == nil {
		return fmt.Errorf("queue batch sender is required")
	}

	for {
		if !producer.waitForBatch(ctx) {
			return producer.flushOnShutdown()
		}
		err := producer.flush(ctx)
		if err != nil {
			slog.Error("flush queued messages", "err", err)
			if !waitForRetry(ctx) {
				return producer.flushOnShutdown()
			}
		}
	}
}

// waitForBatch waits for the first item, then waits for size ten or the timer.
func (producer *batchProducer) waitForBatch(ctx context.Context) bool {
	if producer.pending() == 0 && !waitForWake(ctx, producer.wake) {
		return false
	}
	if producer.pending() >= maxBatchSize {
		return true
	}

	timer := time.NewTimer(producer.flushTimeout)
	defer timer.Stop()
	for producer.pending() < maxBatchSize {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return true
		case <-producer.wake:
		}
	}

	return true
}

// waitForWake lets an empty producer block without polling.
func waitForWake(ctx context.Context, wake <-chan struct{}) bool {
	select {
	case <-ctx.Done():
		return false
	case <-wake:
		return true
	}
}

// waitForRetry limits retries during an SQS outage.
func waitForRetry(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// flushOnShutdown limits the final flush so shutdown cannot hang forever.
func (producer *batchProducer) flushOnShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownFlushLimit)
	defer cancel()

	for producer.pending() > 0 {
		err := producer.flush(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// flush removes one batch before sending so concurrent webhook requests stay fast.
func (producer *batchProducer) flush(ctx context.Context) error {
	actions := producer.takeBatch()
	if len(actions) == 0 {
		return nil
	}

	err := producer.sender.SendMessageBatch(ctx, buildSendMessageBatchRequest(producer.queueURL, actions))
	if err != nil {
		producer.restore(actions)
		return err
	}

	return nil
}

// pending reads the buffer size under its lock to avoid a data race.
func (producer *batchProducer) pending() int {
	producer.mutex.Lock()
	defer producer.mutex.Unlock()

	return len(producer.actions)
}

// takeBatch keeps batches within the SQS maximum of ten entries.
func (producer *batchProducer) takeBatch() []Action {
	producer.mutex.Lock()
	defer producer.mutex.Unlock()

	count := min(len(producer.actions), maxBatchSize)
	actions := append([]Action(nil), producer.actions[:count]...)
	producer.actions = producer.actions[count:]
	return actions
}

// restore puts a failed batch first so FIFO group order is retained on retry.
func (producer *batchProducer) restore(actions []Action) {
	producer.mutex.Lock()
	defer producer.mutex.Unlock()

	producer.actions = append(actions, producer.actions...)
}

// notify wakes the run loop without making webhook requests wait.
func (producer *batchProducer) notify() {
	select {
	case producer.wake <- struct{}{}:
	default:
	}
}
