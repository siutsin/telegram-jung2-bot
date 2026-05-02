package main

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/siutsin/telegram-jung2-bot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Keep this as a subprocess test because main now exits non-zero on fatal
// startup errors, which would terminate the parent test process.
func TestMainExitsNonZeroOnStartupError(t *testing.T) {
	t.Parallel()

	command := exec.Command("go", "run", ".")
	command.Dir = "."
	err := command.Run()

	require.Error(t, err)
	exitError, ok := err.(*exec.ExitError)
	require.True(t, ok)
	assert.NotZero(t, exitError.ExitCode())
}

// This guards the handoff from environment parsing into app startup so missing
// config changes fail here instead of silently shifting runtime wiring tests.
func TestRunLoadsEnvironment(t *testing.T) {
	original := runApp
	t.Cleanup(func() { runApp = original })
	sentinel := errors.New("sentinel")
	called := false
	runApp = func(ctx context.Context, loaded config.Config) error {
		called = true
		assert.Equal(t, "token", loaded.TelegramBotToken)
		assert.Equal(t, "messages", loaded.MessageTable)
		assert.Equal(t, "chats", loaded.ChatIDTable)
		assert.Equal(t, "https://sqs.eu-west-1.amazonaws.com/123/events", loaded.EventQueueURL)
		return sentinel
	}

	err := run(context.Background(), []string{
		"TELEGRAM_BOT_TOKEN=token",
		"MESSAGE_TABLE=messages",
		"CHATID_TABLE=chats",
		"EVENT_QUEUE_URL=https://sqs.eu-west-1.amazonaws.com/123/events",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	assert.True(t, called)
}

func TestRunReturnsConfigError(t *testing.T) {
	err := run(context.Background(), nil)

	require.Error(t, err)
	assert.EqualError(t, err, "TELEGRAM_BOT_TOKEN is required")
}

// This keeps malformed environment entries from polluting config lookup.
func TestEnvMapIgnoresMalformedEntries(t *testing.T) {
	assert.Equal(t, map[string]string{"A": "B=C"}, envMap([]string{"A=B=C", "NOPE"}))
}
