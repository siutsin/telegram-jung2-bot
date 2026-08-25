package queue

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestBatchProducerFlushesFullBatch keeps message-save sends at the SQS batch limit.
func TestBatchProducerFlushesFullBatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		controller := gomock.NewController(t)
		sender := NewMockBatchMessageSender(controller)
		var requests []SendMessageBatchRequest
		sender.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, request SendMessageBatchRequest) error {
			requests = append(requests, request)
			cancel()
			return nil
		})
		producer := NewBatchProducer("fifo-url", sender, time.Hour)
		for index := range maxBatchSize {
			err := producer.Enqueue(ctx, Action{
				Body:                   BodySaveMessage,
				MessageGroupID:         "42",
				MessageDeduplicationID: "42:" + string(rune('a'+index)),
				Attributes:             map[string]string{"action": ActionSaveMessage},
			})
			require.NoError(t, err)
		}
		done := make(chan error)
		go func() { done <- producer.Run(ctx) }()

		synctest.Wait()
		require.NoError(t, <-done)
		require.Len(t, requests, 1)
		assert.Len(t, requests[0].Entries, maxBatchSize)
		assert.Equal(t, "42", requests[0].Entries[0].MessageGroupID)
		assert.Equal(t, "42:a", requests[0].Entries[0].MessageDeduplicationID)
	})
}

// TestBatchProducerFlushesOnTimer keeps low-volume messages from waiting for a full batch.
func TestBatchProducerFlushesOnTimer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		controller := gomock.NewController(t)
		sender := NewMockBatchMessageSender(controller)
		var requests []SendMessageBatchRequest
		sender.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, request SendMessageBatchRequest) error {
			requests = append(requests, request)
			cancel()
			return nil
		})
		producer := NewBatchProducer("fifo-url", sender, time.Second)
		require.NoError(t, producer.Enqueue(ctx, Action{Body: BodySaveMessage}))
		done := make(chan error)
		go func() { done <- producer.Run(ctx) }()

		synctest.Wait()
		require.NoError(t, <-done)
		require.Len(t, requests, 1)
	})
}

// TestBatchProducerFlushesOnShutdown keeps graceful termination from losing buffered messages.
func TestBatchProducerFlushesOnShutdown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	controller := gomock.NewController(t)
	sender := NewMockBatchMessageSender(controller)
	sender.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).Return(nil)
	producer := NewBatchProducer("fifo-url", sender, time.Hour)
	require.NoError(t, producer.Enqueue(context.Background(), Action{Body: BodySaveMessage}))

	require.NoError(t, producer.Run(ctx))
}

// TestBatchProducerRejectsMissingSender prevents accepted work with nowhere to send it.
func TestBatchProducerRejectsMissingSender(t *testing.T) {
	t.Parallel()

	producer := NewBatchProducer("fifo-url", nil, time.Second)
	err := producer.Enqueue(context.Background(), Action{})
	require.EqualError(t, err, "queue batch sender is required")
	err = producer.Run(context.Background())
	require.EqualError(t, err, "queue batch sender is required")
}

// TestNewBatchProducerUsesDefaultTimeout keeps a missing setting from creating a busy flush loop.
func TestNewBatchProducerUsesDefaultTimeout(t *testing.T) {
	producer := NewBatchProducer("fifo-url", NewMockBatchMessageSender(gomock.NewController(t)), 0)

	assert.Equal(t, defaultFlushTimeout, producer.flushTimeout)
}

// TestBatchProducerRestoresFailedBatch keeps transient SQS errors from dropping actions.
func TestBatchProducerRestoresFailedBatch(t *testing.T) {
	t.Parallel()

	controller := gomock.NewController(t)
	sender := NewMockBatchMessageSender(controller)
	sender.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).Return(errors.New("down"))
	producer := NewBatchProducer("fifo-url", sender, time.Second)
	require.NoError(t, producer.Enqueue(context.Background(), Action{Body: BodySaveMessage}))

	err := producer.flush(context.Background())
	require.EqualError(t, err, "down")
	assert.Equal(t, 1, producer.pending())
}

// TestBatchProducerFlushOnShutdownReturnsSendError keeps unsent actions visible after shutdown.
func TestBatchProducerFlushOnShutdownReturnsSendError(t *testing.T) {
	controller := gomock.NewController(t)
	sender := NewMockBatchMessageSender(controller)
	sender.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).Return(errors.New("down"))
	producer := NewBatchProducer("fifo-url", sender, time.Second)
	require.NoError(t, producer.Enqueue(context.Background(), Action{Body: BodySaveMessage}))

	require.EqualError(t, producer.flushOnShutdown(), "down")
}

// TestBatchProducerFlushIgnoresEmptyBuffer keeps wake-up races from sending empty SQS requests.
func TestBatchProducerFlushIgnoresEmptyBuffer(t *testing.T) {
	producer := NewBatchProducer("fifo-url", NewMockBatchMessageSender(gomock.NewController(t)), time.Second)

	require.NoError(t, producer.flush(context.Background()))
}

// TestBatchProducerRetriesAfterAnSQSFailure keeps buffered actions retryable without wall-clock waits.
func TestBatchProducerRetriesAfterAnSQSFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		controller := gomock.NewController(t)
		sender := NewMockBatchMessageSender(controller)
		var requests []SendMessageBatchRequest
		gomock.InOrder(
			sender.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, request SendMessageBatchRequest) error {
				requests = append(requests, request)
				return errors.New("down")
			}),
			sender.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, request SendMessageBatchRequest) error {
				requests = append(requests, request)
				cancel()
				return nil
			}),
		)
		producer := NewBatchProducer("fifo-url", sender, time.Second)
		require.NoError(t, producer.Enqueue(ctx, Action{Body: BodySaveMessage}))
		done := make(chan error)
		go func() { done <- producer.Run(ctx) }()

		synctest.Wait()
		require.NoError(t, <-done)
		assert.Len(t, requests, 2)
	})
}

// TestBatchProducerFlushesAfterCancelledRetry keeps a failed batch on the shutdown path.
func TestBatchProducerFlushesAfterCancelledRetry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	controller := gomock.NewController(t)
	sender := NewMockBatchMessageSender(controller)
	gomock.InOrder(
		sender.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, SendMessageBatchRequest) error {
			cancel()
			return errors.New("down")
		}),
		sender.EXPECT().SendMessageBatch(gomock.Any(), gomock.Any()).Return(nil),
	)
	producer := NewBatchProducer("fifo-url", sender, time.Hour)
	for range maxBatchSize {
		require.NoError(t, producer.Enqueue(ctx, Action{Body: BodySaveMessage}))
	}

	require.NoError(t, producer.Run(ctx))
}

// TestWaitForRetryStopsOnCancellation keeps an outage from delaying shutdown.
func TestWaitForRetryStopsOnCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.False(t, waitForRetry(ctx))
}

// TestBatchProducerStopsIdleOnCancellation keeps an empty queue from delaying shutdown.
func TestBatchProducerStopsIdleOnCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	producer := NewBatchProducer("fifo-url", NewMockBatchMessageSender(gomock.NewController(t)), time.Hour)

	require.NoError(t, producer.Run(ctx))
}

// TestWaitForWakeAcceptsAnEnqueueSignal lets an idle producer start its flush timer.
func TestWaitForWakeAcceptsAnEnqueueSignal(t *testing.T) {
	t.Parallel()

	wake := make(chan struct{}, 1)
	wake <- struct{}{}

	assert.True(t, waitForWake(context.Background(), wake))
}

// TestBatchProducerWaitsForTheLastBatchEntry avoids delaying a completed batch until its timer fires.
func TestBatchProducerWaitsForTheLastBatchEntry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		producer := NewBatchProducer("fifo-url", NewMockBatchMessageSender(gomock.NewController(t)), time.Hour)
		for range maxBatchSize - 1 {
			require.NoError(t, producer.Enqueue(t.Context(), Action{Body: BodySaveMessage}))
		}
		done := make(chan bool)
		go func() { done <- producer.waitForBatch(t.Context()) }()

		synctest.Wait()
		require.NoError(t, producer.Enqueue(t.Context(), Action{Body: BodySaveMessage}))
		synctest.Wait()
		assert.True(t, <-done)
	})
}

// TestBuildSendMessageBatchRequestPreservesFIFOFields keeps deduplication metadata on every SQS entry.
func TestBuildSendMessageBatchRequestPreservesFIFOFields(t *testing.T) {
	t.Parallel()

	request := buildSendMessageBatchRequest("fifo-url", []Action{{
		Body:                   BodySaveMessage,
		MessageGroupID:         "42",
		MessageDeduplicationID: "42:7",
	}})

	require.Len(t, request.Entries, 1)
	assert.Equal(t, SendMessageBatchEntry{
		ID:                     "0",
		MessageBody:            BodySaveMessage,
		MessageAttributes:      map[string]SendMessageAttribute{},
		MessageGroupID:         "42",
		MessageDeduplicationID: "42:7",
	}, request.Entries[0])
}
