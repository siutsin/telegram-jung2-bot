// Package config parses and validates service configuration.
package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
)

var dynamoDBTableNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,255}$`)

// Config contains validated startup configuration.
type Config struct {
	AWSRegion           string
	TelegramBotToken    string
	MessageTable        string
	ChatIDTable         string
	EventQueueURL       string
	LogLevel            string
	ServerAddress       string
	ScaleUpReadCapacity int
}

// Load validates configuration from an environment map.
func Load(env map[string]string) (Config, error) {
	config := Config{
		AWSRegion:           valueOrDefault(env, "AWS_REGION", "eu-west-1"),
		LogLevel:            valueOrDefault(env, "LOG_LEVEL", "info"),
		ServerAddress:       valueOrDefault(env, "SERVER_ADDRESS", ":3000"),
		TelegramBotToken:    env["TELEGRAM_BOT_TOKEN"],
		MessageTable:        env["MESSAGE_TABLE"],
		ChatIDTable:         env["CHATID_TABLE"],
		EventQueueURL:       env["EVENT_QUEUE_URL"],
		ScaleUpReadCapacity: 1,
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

	if raw := env["SCALE_UP_READ_CAPACITY"]; raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return Config{}, fmt.Errorf("SCALE_UP_READ_CAPACITY must be a positive integer")
		}
		config.ScaleUpReadCapacity = value
	}

	return config, nil
}

func valueOrDefault(env map[string]string, key string, fallback string) string {
	if env[key] == "" {
		return fallback
	}

	return env[key]
}

func validateTableName(key string, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", key)
	}
	if !dynamoDBTableNamePattern.MatchString(value) {
		return fmt.Errorf("%s is not a valid DynamoDB table name", key)
	}

	return nil
}

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
