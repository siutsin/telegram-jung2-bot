package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBuildsConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(env map[string]string)
		check  func(t *testing.T, config Config)
	}{
		{
			name: "defaults",
			check: func(t *testing.T, config Config) {
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
			},
		},
		{
			name: "overrides",
			mutate: func(env map[string]string) {
				env["AWS_REGION"] = "ap-east-1"
				env["LOG_LEVEL"] = "debug"
				env["STAGE"] = "prod"
				env["WEBHOOK_SECRET_TOKEN"] = "webhook-secret"
				env["SCHEDULER_SECRET_TOKEN"] = "scheduler-secret"
				env["SERVER_ADDRESS"] = ":8080"
				env["SCALE_UP_READ_CAPACITY"] = "5"
				env["TELEGRAM_API_BASE_URL"] = "http://localhost:8081"
				env["HTTP_TIMEOUT_SECONDS"] = "3"
				env["SHUTDOWN_TIMEOUT_SECONDS"] = "4"
			},
			check: func(t *testing.T, config Config) {
				assert.Equal(t, "ap-east-1", config.AWSRegion)
				assert.Equal(t, "debug", config.LogLevel)
				assert.Equal(t, "prod", config.Stage)
				assert.Equal(t, ":8080", config.ServerAddress)
				assert.Equal(t, 5, config.ScaleUpReadCapacity)
				assert.Equal(t, "http://localhost:8081", config.TelegramAPIBaseURL)
				assert.Equal(t, 3*time.Second, config.HTTPTimeout)
				assert.Equal(t, 4*time.Second, config.ShutdownTimeout)
			},
		},
		{
			name: "docker bind default",
			mutate: func(env map[string]string) {
				env["DOCKER"] = "1"
			},
			check: func(t *testing.T, config Config) {
				assert.Equal(t, "0.0.0.0:3000", config.ServerAddress)
			},
		},
		{
			name: "local endpoint override",
			mutate: func(env map[string]string) {
				env["AWS_ENDPOINT_URL"] = "http://localhost:4566"
			},
			check: func(t *testing.T, config Config) {
				assert.Equal(t, "http://localhost:4566", config.AWSEndpointURL)
			},
		},
		{
			name: "normalises stage",
			mutate: func(env map[string]string) {
				env["STAGE"] = " Prod "
				env["WEBHOOK_SECRET_TOKEN"] = "webhook-secret"
				env["SCHEDULER_SECRET_TOKEN"] = "scheduler-secret"
			},
			check: func(t *testing.T, config Config) {
				assert.Equal(t, "prod", config.Stage)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := validEnv()
			if test.mutate != nil {
				test.mutate(env)
			}

			config, err := Load(env)
			require.NoError(t, err)
			test.check(t, config)
		})
	}
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

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	t.Parallel()

	env := validEnv()
	env["LOG_LEVEL"] = "trace"

	_, err := Load(env)
	require.Error(t, err)
	assert.EqualError(t, err, "LOG_LEVEL must be one of debug, info, warn, warning, or error")
}

func TestLoadRejectsProductionEndpointOverride(t *testing.T) {
	t.Parallel()

	env := validEnv()
	env["STAGE"] = "prod"
	env["WEBHOOK_SECRET_TOKEN"] = "webhook-secret"
	env["SCHEDULER_SECRET_TOKEN"] = "scheduler-secret"
	env["AWS_ENDPOINT_URL"] = "http://localhost:4566"

	_, err := Load(env)
	require.Error(t, err)
	assert.EqualError(t, err, "AWS_ENDPOINT_URL is not allowed for stage \"prod\"")
}

func TestServerAddressIgnoresFalseDockerFlag(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "127.0.0.1:3000", serverAddress("", "false"))
	assert.Equal(t, "127.0.0.1:3000", serverAddress("", "0"))
}

func TestLoadRequiresProductionSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(env map[string]string)
		wantErr string
	}{
		{
			name: "missing webhook secret",
			mutate: func(env map[string]string) {
				env["STAGE"] = "prod"
				env["SCHEDULER_SECRET_TOKEN"] = "scheduler-secret"
			},
			wantErr: "WEBHOOK_SECRET_TOKEN is required for stage \"prod\"",
		},
		{
			name: "missing scheduler secret",
			mutate: func(env map[string]string) {
				env["STAGE"] = "prod"
				env["WEBHOOK_SECRET_TOKEN"] = "webhook-secret"
			},
			wantErr: "SCHEDULER_SECRET_TOKEN is required for stage \"prod\"",
		},
		{
			name: "dev allows missing secrets",
			mutate: func(env map[string]string) {
				env["STAGE"] = "dev"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			env := validEnv()
			if test.mutate != nil {
				test.mutate(env)
			}

			_, err := Load(env)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
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
