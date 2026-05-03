// Package config parses and validates service configuration.
package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	caarlosenv "github.com/caarlos0/env/v11"
)

var dynamoDBTableNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,255}$`)

// Config contains validated startup configuration.
type Config struct {
	AWSRegion           string
	TelegramBotToken    string
	MessageTable        string
	ChatIDTable         string
	EventQueueURL       string
	AWSEndpointURL      string
	TelegramAPIBaseURL  string
	LogLevel            string
	Stage               string
	ServerAddress       string
	HTTPTimeout         time.Duration
	ShutdownTimeout     time.Duration
	ScaleUpReadCapacity int
}

type rawConfig struct {
	AWSRegion              string `env:"AWS_REGION" envDefault:"eu-west-1"`
	TelegramBotToken       string `env:"TELEGRAM_BOT_TOKEN"`
	MessageTable           string `env:"MESSAGE_TABLE"`
	ChatIDTable            string `env:"CHATID_TABLE"`
	EventQueueURL          string `env:"EVENT_QUEUE_URL"`
	AWSEndpointURL         string `env:"AWS_ENDPOINT_URL"`
	TelegramAPIBaseURL     string `env:"TELEGRAM_API_BASE_URL" envDefault:"https://api.telegram.org"`
	LogLevel               string `env:"LOG_LEVEL" envDefault:"info"`
	Stage                  string `env:"STAGE" envDefault:"dev"`
	ServerAddress          string `env:"SERVER_ADDRESS"`
	Docker                 string `env:"DOCKER"`
	HTTPTimeoutSeconds     string `env:"HTTP_TIMEOUT_SECONDS"`
	ShutdownTimeoutSeconds string `env:"SHUTDOWN_TIMEOUT_SECONDS"`
	ScaleUpReadCapacity    string `env:"SCALE_UP_READ_CAPACITY"`
}

// Load validates configuration from an environment map.
func Load(env map[string]string) (Config, error) {
	raw, err := parseRawConfig(env)
	if err != nil {
		return Config{}, err
	}

	config := configFromRaw(raw)
	if err := validateConfig(config); err != nil {
		return Config{}, err
	}
	applyScaleUpReadCapacity(&config, raw.ScaleUpReadCapacity)
	if err := applyTimeouts(&config, raw); err != nil {
		return Config{}, err
	}

	return config, nil
}

// LoadEnviron validates configuration from process-style environment entries.
func LoadEnviron(environ []string) (Config, error) {
	return Load(caarlosenv.ToMap(environ))
}

// parseRawConfig decodes environment variables into the raw config shape.
func parseRawConfig(env map[string]string) (rawConfig, error) {
	raw, err := caarlosenv.ParseAsWithOptions[rawConfig](caarlosenv.Options{Environment: env})
	if err != nil {
		return rawConfig{}, fmt.Errorf("parse environment: %w", err)
	}

	return raw, nil
}

// configFromRaw builds defaulted runtime config from raw environment values.
func configFromRaw(raw rawConfig) Config {
	return Config{
		AWSRegion:           raw.AWSRegion,
		LogLevel:            raw.LogLevel,
		Stage:               raw.Stage,
		ServerAddress:       serverAddress(raw.ServerAddress, raw.Docker),
		TelegramAPIBaseURL:  raw.TelegramAPIBaseURL,
		TelegramBotToken:    raw.TelegramBotToken,
		MessageTable:        raw.MessageTable,
		ChatIDTable:         raw.ChatIDTable,
		EventQueueURL:       raw.EventQueueURL,
		AWSEndpointURL:      raw.AWSEndpointURL,
		HTTPTimeout:         10 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		ScaleUpReadCapacity: 0,
	}
}

// validateConfig checks required startup settings before clients are built.
func validateConfig(config Config) error {
	if config.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if err := validateTableName("MESSAGE_TABLE", config.MessageTable); err != nil {
		return err
	}
	if err := validateTableName("CHATID_TABLE", config.ChatIDTable); err != nil {
		return err
	}
	if err := validateURL("EVENT_QUEUE_URL", config.EventQueueURL); err != nil {
		return err
	}
	if err := validateOptionalURL("AWS_ENDPOINT_URL", config.AWSEndpointURL); err != nil {
		return err
	}
	if err := validateURL("TELEGRAM_API_BASE_URL", config.TelegramAPIBaseURL); err != nil {
		return err
	}

	return nil
}

// applyScaleUpReadCapacity keeps invalid scale-up values at the default.
func applyScaleUpReadCapacity(config *Config, raw string) {
	if raw == "" {
		return
	}

	value, err := strconv.Atoi(raw)
	if err == nil && value > 0 {
		config.ScaleUpReadCapacity = value
	}
}

// applyTimeouts overrides default timeouts from validated raw values.
func applyTimeouts(config *Config, raw rawConfig) error {
	if err := applyTimeout(&config.HTTPTimeout, "HTTP_TIMEOUT_SECONDS", raw.HTTPTimeoutSeconds); err != nil {
		return err
	}
	if err := applyTimeout(&config.ShutdownTimeout, "SHUTDOWN_TIMEOUT_SECONDS", raw.ShutdownTimeoutSeconds); err != nil {
		return err
	}

	return nil
}

// applyTimeout replaces a default timeout when the raw value is set.
func applyTimeout(target *time.Duration, key string, raw string) error {
	if raw == "" {
		return nil
	}

	timeout, err := parsePositiveSeconds(key, raw)
	if err != nil {
		return err
	}
	*target = timeout

	return nil
}

// serverAddress returns the default bind address for the current environment.
func serverAddress(value string, docker string) string {
	if value != "" {
		return value
	}
	if docker != "" {
		return "0.0.0.0:3000"
	}

	return "127.0.0.1:3000"
}

// validateTableName checks a DynamoDB table name.
func validateTableName(key string, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", key)
	}
	if !dynamoDBTableNamePattern.MatchString(value) {
		return fmt.Errorf("%s is not a valid DynamoDB table name", key)
	}

	return nil
}

// validateURL checks that value is an absolute URL.
func validateURL(key string, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", key)
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", key)
	}

	return nil
}

// validateOptionalURL checks an optional absolute URL.
func validateOptionalURL(key string, value string) error {
	if value == "" {
		return nil
	}

	return validateURL(key, value)
}

// parsePositiveSeconds parses a positive timeout in seconds.
func parsePositiveSeconds(key string, raw string) (time.Duration, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}

	return time.Duration(value) * time.Second, nil
}
