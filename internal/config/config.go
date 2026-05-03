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

const (
	defaultHTTPTimeout     = 10 * time.Second
	defaultShutdownTimeout = 10 * time.Second
)

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
}

// Load validates configuration from an environment map.
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
func LoadEnviron(environ []string) (Config, error) {
	return Load(caarlosenv.ToMap(environ))
}

// parseRawConfig decodes environment variables into the raw config shape.
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
func configFromRaw(raw rawConfig) (Config, error) {
	httpTimeout := defaultHTTPTimeout
	if raw.HTTPTimeoutSeconds != "" {
		parsedTimeout, err := parsePositiveSeconds("HTTP_TIMEOUT_SECONDS", raw.HTTPTimeoutSeconds)
		if err != nil {
			return Config{}, err
		}
		httpTimeout = parsedTimeout
	}

	shutdownTimeout := defaultShutdownTimeout
	if raw.ShutdownTimeoutSeconds != "" {
		parsedTimeout, err := parsePositiveSeconds("SHUTDOWN_TIMEOUT_SECONDS", raw.ShutdownTimeoutSeconds)
		if err != nil {
			return Config{}, err
		}
		shutdownTimeout = parsedTimeout
	}

	scaleUpReadCapacity := 0
	parsedScaleUpReadCapacity, err := strconv.Atoi(raw.ScaleUpReadCapacity)
	if err == nil && parsedScaleUpReadCapacity > 0 {
		scaleUpReadCapacity = parsedScaleUpReadCapacity
	}

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
		HTTPTimeout:         httpTimeout,
		ShutdownTimeout:     shutdownTimeout,
		ScaleUpReadCapacity: scaleUpReadCapacity,
	}, nil
}

// validateConfig checks required startup settings before clients are built.
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
	if !dynamoDBTableNamePattern.MatchString(value) {
		return fmt.Errorf("%s is not a valid DynamoDB table name", key)
	}

	return nil
}

// validateURL checks that value is an absolute URL.
func validateURL(key string, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", key)
	}

	return nil
}

// parsePositiveSeconds parses a positive timeout in seconds.
func parsePositiveSeconds(key string, raw string) (time.Duration, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}

	return time.Duration(value) * time.Second, nil
}
