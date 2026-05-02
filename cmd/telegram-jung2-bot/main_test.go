package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainRuns(t *testing.T) {
	main()
}

func TestRunLoadsEnvironment(t *testing.T) {
	err := run(context.Background(), []string{
		"TELEGRAM_BOT_TOKEN=token",
		"MESSAGE_TABLE=messages",
		"CHATID_TABLE=chats",
		"EVENT_QUEUE_URL=https://sqs.eu-west-1.amazonaws.com/123/events",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create HTTP server")
}

func TestRunReturnsConfigError(t *testing.T) {
	err := run(context.Background(), nil)

	require.Error(t, err)
	assert.EqualError(t, err, "TELEGRAM_BOT_TOKEN is required")
}

func TestEnvMapIgnoresMalformedEntries(t *testing.T) {
	assert.Equal(t, map[string]string{"A": "B=C"}, envMap([]string{"A=B=C", "NOPE"}))
}
