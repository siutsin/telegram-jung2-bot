package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAcceptsValidConfig(t *testing.T) {
	config, err := Load(validEnv())
	require.NoError(t, err)

	assert.Equal(t, "eu-west-1", config.AWSRegion)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, ":3000", config.ServerAddress)
	assert.Equal(t, "messages-dev", config.MessageTable)
	assert.Equal(t, "chat-id-dev", config.ChatIDTable)
	assert.Equal(t, 1, config.ScaleUpReadCapacity)
}

func TestLoadUsesOverrides(t *testing.T) {
	env := validEnv()
	env["AWS_REGION"] = "ap-east-1"
	env["LOG_LEVEL"] = "debug"
	env["SERVER_ADDRESS"] = ":8080"
	env["SCALE_UP_READ_CAPACITY"] = "5"

	config, err := Load(env)
	require.NoError(t, err)

	assert.Equal(t, "ap-east-1", config.AWSRegion)
	assert.Equal(t, "debug", config.LogLevel)
	assert.Equal(t, ":8080", config.ServerAddress)
	assert.Equal(t, 5, config.ScaleUpReadCapacity)
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
	env := validEnv()
	env["EVENT_QUEUE_URL"] = "not-a-url"

	_, err := Load(env)
	require.Error(t, err)
}

func TestLoadRejectsInvalidScaleUpReadCapacity(t *testing.T) {
	tests := []string{"0", "-1", "many"}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			env := validEnv()
			env["SCALE_UP_READ_CAPACITY"] = value

			_, err := Load(env)
			require.Error(t, err)
		})
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"TELEGRAM_BOT_TOKEN": "token",
		"MESSAGE_TABLE":      "messages-dev",
		"CHATID_TABLE":       "chat-id-dev",
		"EVENT_QUEUE_URL":    "https://sqs.eu-west-1.amazonaws.com/123/events",
	}
}
