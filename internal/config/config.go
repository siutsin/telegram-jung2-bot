// Package config parses and validates service configuration.
package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	caarlosenv "github.com/caarlos0/env/v11"
)

var dynamoDBTableNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,255}$`)

const (
	defaultHTTPTimeout     = 10 * time.Second
	defaultShutdownTimeout = 10 * time.Second
)

// Config contains validated startup configuration.
type Config struct {
	AWSRegion            string
	TelegramBotToken     string
	MessageTable         string
	ChatIDTable          string
	EventQueueURL        string
	AWSEndpointURL       string
	TelegramAPIBaseURL   string
	LogLevel             string
	Stage                string
	ServerAddress        string
	HTTPTimeout          time.Duration
	ShutdownTimeout      time.Duration
	ScaleUpReadCapacity  int
	WebhookSecretToken   string
	SchedulerSecretToken string
}

type rawConfig struct {
	AWSRegion              string `env:"AWS_REGION" envDefault:"eu-west-1"`
	TelegramBotToken       string `env:"TELEGRAM_BOT_TOKEN,required"`
	MessageTable           string `env:"MESSAGE_TABLE,required"`
	ChatIDTable            string `env:"CHATID_TABLE,required"`
	EventQueueURL          string `env:"EVENT_QUEUE_URL,required"`
	AWSEndpointURL         string `env:"AWS_ENDPOINT_URL"`
	TelegramAPIBaseURL     string `env:"TELEGRAM_API_BASE_URL" envDefault:"https://api.telegram.org"`
	LogLevel               string `env:"LOG_LEVEL" envDefault:"info"`
	Stage                  string `env:"STAGE" envDefault:"dev"`
	ServerAddress          string `env:"SERVER_ADDRESS"`
	Docker                 string `env:"DOCKER"`
	HTTPTimeoutSeconds     string `env:"HTTP_TIMEOUT_SECONDS"`
	ShutdownTimeoutSeconds string `env:"SHUTDOWN_TIMEOUT_SECONDS"`
	ScaleUpReadCapacity    string `env:"SCALE_UP_READ_CAPACITY"`
	WebhookSecretToken     string `env:"WEBHOOK_SECRET_TOKEN"`
	SchedulerSecretToken   string `env:"SCHEDULER_SECRET_TOKEN"`
}

// Load validates configuration from an environment map.
// For example, it turns "HTTP_TIMEOUT_SECONDS=5" into HTTPTimeout=5*time.Second.
func Load(env map[string]string) (Config, error) {
	raw, err := parseRawConfig(env)
	if err != nil {
		return Config{}, err
	}

	config, err := configFromRaw(raw)
	if err != nil {
		return Config{}, err
	}
	err = validateConfig(config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}

// LoadEnviron validates configuration from process-style environment entries.
// For example, []string{"STAGE=prod"} becomes Config{Stage: "prod"}.
func LoadEnviron(environ []string) (Config, error) {
	return Load(caarlosenv.ToMap(environ))
}

// parseRawConfig decodes environment variables into the raw config shape.
// For example, "HTTP_TIMEOUT_SECONDS=5" stays as raw.HTTPTimeoutSeconds == "5".
func parseRawConfig(env map[string]string) (rawConfig, error) {
	raw, err := caarlosenv.ParseAsWithOptions[rawConfig](caarlosenv.Options{
		Environment: env,
	})
	if err != nil {
		return rawConfig{}, fmt.Errorf("parse environment: %w", err)
	}

	return raw, nil
}

// configFromRaw builds defaulted runtime config from raw environment values.
// For example, it turns raw timeout strings into time.Duration values and
// fills the default server address when none is set.
func configFromRaw(raw rawConfig) (Config, error) {
	httpTimeout := defaultHTTPTimeout
	if raw.HTTPTimeoutSeconds != "" {
		parsedTimeout, parseErr := parsePositiveSeconds("HTTP_TIMEOUT_SECONDS", raw.HTTPTimeoutSeconds)
		if parseErr != nil {
			return Config{}, parseErr
		}
		httpTimeout = parsedTimeout
	}

	shutdownTimeout := defaultShutdownTimeout
	if raw.ShutdownTimeoutSeconds != "" {
		parsedTimeout, parseErr := parsePositiveSeconds("SHUTDOWN_TIMEOUT_SECONDS", raw.ShutdownTimeoutSeconds)
		if parseErr != nil {
			return Config{}, parseErr
		}
		shutdownTimeout = parsedTimeout
	}

	scaleUpReadCapacity := 0
	parsedScaleUpReadCapacity, err := strconv.Atoi(raw.ScaleUpReadCapacity)
	if err == nil && parsedScaleUpReadCapacity > 0 {
		scaleUpReadCapacity = parsedScaleUpReadCapacity
	}

	return Config{
		AWSRegion:            raw.AWSRegion,
		LogLevel:             raw.LogLevel,
		Stage:                strings.ToLower(strings.TrimSpace(raw.Stage)),
		ServerAddress:        serverAddress(raw.ServerAddress, raw.Docker),
		TelegramAPIBaseURL:   raw.TelegramAPIBaseURL,
		TelegramBotToken:     raw.TelegramBotToken,
		MessageTable:         raw.MessageTable,
		ChatIDTable:          raw.ChatIDTable,
		EventQueueURL:        raw.EventQueueURL,
		AWSEndpointURL:       raw.AWSEndpointURL,
		HTTPTimeout:          httpTimeout,
		ShutdownTimeout:      shutdownTimeout,
		ScaleUpReadCapacity:  scaleUpReadCapacity,
		WebhookSecretToken:   raw.WebhookSecretToken,
		SchedulerSecretToken: raw.SchedulerSecretToken,
	}, nil
}

// validateConfig checks required startup settings before clients are built.
// For example, it rejects a relative EVENT_QUEUE_URL like "/queue".
func validateConfig(config Config) error {
	err := validateTableName("MESSAGE_TABLE", config.MessageTable)
	if err != nil {
		return err
	}
	err = validateTableName("CHATID_TABLE", config.ChatIDTable)
	if err != nil {
		return err
	}
	err = validateURL("EVENT_QUEUE_URL", config.EventQueueURL)
	if err != nil {
		return err
	}
	if config.AWSEndpointURL != "" {
		err = validateURL("AWS_ENDPOINT_URL", config.AWSEndpointURL)
		if err != nil {
			return err
		}
	}
	err = validateURL("TELEGRAM_API_BASE_URL", config.TelegramAPIBaseURL)
	if err != nil {
		return err
	}
	err = validateLogLevel(config.LogLevel)
	if err != nil {
		return err
	}

	return requireProductionSecrets(config)
}

// requireProductionSecrets rejects non-dev stages that run without auth tokens.
// For example, STAGE=prod without WEBHOOK_SECRET_TOKEN fails config validation.
func requireProductionSecrets(config Config) error {
	if isLocalStage(config.Stage) {
		return nil
	}
	if strings.TrimSpace(config.WebhookSecretToken) == "" {
		return fmt.Errorf("WEBHOOK_SECRET_TOKEN is required for stage %q", config.Stage)
	}
	if strings.TrimSpace(config.SchedulerSecretToken) == "" {
		return fmt.Errorf("SCHEDULER_SECRET_TOKEN is required for stage %q", config.Stage)
	}
	if config.AWSEndpointURL != "" {
		return fmt.Errorf("AWS_ENDPOINT_URL is not allowed for stage %q", config.Stage)
	}

	return nil
}

// isLocalStage reports whether stage is exempt from production secret checks.
// For example, "dev" and " local " both return true.
func isLocalStage(stage string) bool {
	stage = strings.ToLower(strings.TrimSpace(stage))

	return stage == "dev" || stage == "test" || stage == "local"
}

// validateLogLevel checks LOG_LEVEL against supported slog levels.
// For example, "debug" is valid while "trace" is rejected.
func validateLogLevel(level string) error {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "debug", "info", "warn", "warning", "error":
		return nil
	default:
		return fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, warning, or error")
	}
}

// serverAddress returns the default bind address for the current environment.
// For example, empty input becomes "127.0.0.1:3000" or "0.0.0.0:3000" in Docker.
func serverAddress(value string, docker string) string {
	if value != "" {
		return value
	}
	if dockerEnabled(docker) {
		return "0.0.0.0:3000"
	}

	return "127.0.0.1:3000"
}

// dockerEnabled reports whether DOCKER requests the container bind address.
// For example, DOCKER=1 and DOCKER=true return true while DOCKER=false does not.
func dockerEnabled(value string) bool {
	enabled, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false
	}

	return enabled
}

// validateTableName checks a DynamoDB table name.
// For example, "messages-prod" is valid, while "bad/table" is not.
func validateTableName(key string, value string) error {
	if !dynamoDBTableNamePattern.MatchString(value) {
		return fmt.Errorf("%s is not a valid DynamoDB table name", key)
	}

	return nil
}

// validateURL checks that value is an absolute URL.
// For example, "https://example.com/queue" is valid, while "/queue" is not.
func validateURL(key string, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", key)
	}

	return nil
}

// parsePositiveSeconds parses a positive timeout in seconds.
// For example, "5" becomes 5*time.Second.
func parsePositiveSeconds(key string, raw string) (time.Duration, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}

	return time.Duration(value) * time.Second, nil
}
