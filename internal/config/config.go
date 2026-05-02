// Package config parses and validates service configuration.
package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"
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

// Load validates configuration from an environment map.
func Load(env map[string]string) (Config, error) {
	config := Config{
		AWSRegion:           valueOrDefault(env, "AWS_REGION", "eu-west-1"),
		LogLevel:            valueOrDefault(env, "LOG_LEVEL", "info"),
		Stage:               valueOrDefault(env, "STAGE", "dev"),
		ServerAddress:       serverAddress(env),
		TelegramAPIBaseURL:  valueOrDefault(env, "TELEGRAM_API_BASE_URL", "https://api.telegram.org"),
		TelegramBotToken:    env["TELEGRAM_BOT_TOKEN"],
		MessageTable:        env["MESSAGE_TABLE"],
		ChatIDTable:         env["CHATID_TABLE"],
		EventQueueURL:       env["EVENT_QUEUE_URL"],
		AWSEndpointURL:      env["AWS_ENDPOINT_URL"],
		HTTPTimeout:         10 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		ScaleUpReadCapacity: 0,
	}

	if config.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if err := validateTableName("MESSAGE_TABLE", config.MessageTable); err != nil {
		return Config{}, err
	}
	if err := validateTableName("CHATID_TABLE", config.ChatIDTable); err != nil {
		return Config{}, err
	}
	if err := validateURL("EVENT_QUEUE_URL", config.EventQueueURL); err != nil {
		return Config{}, err
	}
	if err := validateOptionalURL("AWS_ENDPOINT_URL", config.AWSEndpointURL); err != nil {
		return Config{}, err
	}
	if err := validateURL("TELEGRAM_API_BASE_URL", config.TelegramAPIBaseURL); err != nil {
		return Config{}, err
	}

	if raw := env["SCALE_UP_READ_CAPACITY"]; raw != "" {
		value, err := strconv.Atoi(raw)
		if err == nil && value > 0 {
			config.ScaleUpReadCapacity = value
		}
	}
	if raw := env["HTTP_TIMEOUT_SECONDS"]; raw != "" {
		timeout, err := parsePositiveSeconds("HTTP_TIMEOUT_SECONDS", raw)
		if err != nil {
			return Config{}, err
		}
		config.HTTPTimeout = timeout
	}
	if raw := env["SHUTDOWN_TIMEOUT_SECONDS"]; raw != "" {
		timeout, err := parsePositiveSeconds("SHUTDOWN_TIMEOUT_SECONDS", raw)
		if err != nil {
			return Config{}, err
		}
		config.ShutdownTimeout = timeout
	}

	return config, nil
}

// valueOrDefault returns fallback when env[key] is empty.
func valueOrDefault(env map[string]string, key string, fallback string) string {
	if env[key] == "" {
		return fallback
	}

	return env[key]
}

// serverAddress returns the default bind address for the current environment.
func serverAddress(env map[string]string) string {
	if env["SERVER_ADDRESS"] != "" {
		return env["SERVER_ADDRESS"]
	}
	if env["DOCKER"] != "" {
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
