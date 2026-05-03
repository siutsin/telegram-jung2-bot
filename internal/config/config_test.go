package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAcceptsValidConfig(t *testing.T) {
	config, err := Load(validEnv())
	require.NoError(t, err)

	assert.Equal(t, "eu-west-1", config.AWSRegion)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, "dev", config.Stage)
	assert.Equal(t, "127.0.0.1:3000", config.ServerAddress)
	assert.Equal(t, "messages-dev", config.MessageTable)
	assert.Equal(t, "chat-id-dev", config.ChatIDTable)
	assert.Empty(t, config.AWSEndpointURL)
	assert.Equal(t, "https://api.telegram.org", config.TelegramAPIBaseURL)
	assert.Equal(t, 10*time.Second, config.HTTPTimeout)
	assert.Equal(t, 10*time.Second, config.ShutdownTimeout)
	assert.Equal(t, 0, config.ScaleUpReadCapacity)
}

func TestLoadUsesOverrides(t *testing.T) {
	env := validEnv()
	env["AWS_REGION"] = "ap-east-1"
	env["LOG_LEVEL"] = "debug"
	env["STAGE"] = "prod"
	env["SERVER_ADDRESS"] = ":8080"
	env["SCALE_UP_READ_CAPACITY"] = "5"
	env["AWS_ENDPOINT_URL"] = "http://localhost:4566"
	env["TELEGRAM_API_BASE_URL"] = "http://localhost:8081"
	env["HTTP_TIMEOUT_SECONDS"] = "3"
	env["SHUTDOWN_TIMEOUT_SECONDS"] = "4"

	config, err := Load(env)
	require.NoError(t, err)

	assert.Equal(t, "ap-east-1", config.AWSRegion)
	assert.Equal(t, "debug", config.LogLevel)
	assert.Equal(t, "prod", config.Stage)
	assert.Equal(t, ":8080", config.ServerAddress)
	assert.Equal(t, 5, config.ScaleUpReadCapacity)
	assert.Equal(t, "http://localhost:4566", config.AWSEndpointURL)
	assert.Equal(t, "http://localhost:8081", config.TelegramAPIBaseURL)
	assert.Equal(t, 3*time.Second, config.HTTPTimeout)
	assert.Equal(t, 4*time.Second, config.ShutdownTimeout)
}

func TestLoadUsesDockerBindDefault(t *testing.T) {
	env := validEnv()
	env["DOCKER"] = "1"

	config, err := Load(env)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0:3000", config.ServerAddress)
}

func TestLoadRejectsMissingRequiredValues(t *testing.T) {
	tests := []string{
		"TELEGRAM_BOT_TOKEN",
		"MESSAGE_TABLE",
		"CHATID_TABLE",
		"EVENT_QUEUE_URL",
	}

	for _, key := range tests {
		t.Run(key, func(t *testing.T) {
			env := validEnv()
			delete(env, key)

			_, err := Load(env)
			require.Error(t, err)
		})
	}
}

func TestLoadRejectsInvalidTableNamesBeforeClientsAreBuilt(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{key: "MESSAGE_TABLE", value: "no"},
		{key: "MESSAGE_TABLE", value: "bad/name"},
		{key: "CHATID_TABLE", value: "bad name"},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			env := validEnv()
			env[test.key] = test.value

			_, err := Load(env)
			require.Error(t, err)
		})
	}
}

func TestLoadRejectsInvalidQueueURL(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{key: "EVENT_QUEUE_URL", value: "not-a-url"},
		{key: "AWS_ENDPOINT_URL", value: "localhost:4566"},
		{key: "TELEGRAM_API_BASE_URL", value: "localhost:8081"},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			env := validEnv()
			env[test.key] = test.value

			_, err := Load(env)
			require.Error(t, err)
		})
	}
}

func TestLoadFallsBackForInvalidScaleUpReadCapacity(t *testing.T) {
	tests := []string{"0", "-1", "many"}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			env := validEnv()
			env["SCALE_UP_READ_CAPACITY"] = value

			config, err := Load(env)
			require.NoError(t, err)
			assert.Equal(t, 0, config.ScaleUpReadCapacity)
		})
	}
}

func TestLoadRejectsInvalidTimeouts(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{key: "HTTP_TIMEOUT_SECONDS", value: "0"},
		{key: "HTTP_TIMEOUT_SECONDS", value: "-1"},
		{key: "HTTP_TIMEOUT_SECONDS", value: "many"},
		{key: "SHUTDOWN_TIMEOUT_SECONDS", value: "0"},
		{key: "SHUTDOWN_TIMEOUT_SECONDS", value: "-1"},
		{key: "SHUTDOWN_TIMEOUT_SECONDS", value: "many"},
	}

	for _, test := range tests {
		t.Run(test.key+"="+test.value, func(t *testing.T) {
			env := validEnv()
			env[test.key] = test.value

			_, err := Load(env)
			require.Error(t, err)
		})
	}
}

// This keeps malformed environment entries from polluting process boot config.
func TestLoadEnvironIgnoresMalformedEntries(t *testing.T) {
	config, err := LoadEnviron([]string{
		"TELEGRAM_BOT_TOKEN=token",
		"MESSAGE_TABLE=messages-dev",
		"CHATID_TABLE=chat-id-dev",
		"EVENT_QUEUE_URL=https://sqs.eu-west-1.amazonaws.com/123/events",
		"NOPE",
	})

	require.NoError(t, err)
	assert.Equal(t, "token", config.TelegramBotToken)
}

func validEnv() map[string]string {
	return map[string]string{
		"TELEGRAM_BOT_TOKEN": "token",
		"MESSAGE_TABLE":      "messages-dev",
		"CHATID_TABLE":       "chat-id-dev",
		"EVENT_QUEUE_URL":    "https://sqs.eu-west-1.amazonaws.com/123/events",
	}
}
