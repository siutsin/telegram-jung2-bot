package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	bot "github.com/siutsin/telegram-jung2-bot/internal"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

// main starts the bot process.
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err := run(ctx)
	if err != nil {
		slog.Error("telegram-jung2-bot stopped", "error", err)
		os.Exit(1)
	}
}

// run loads config, assembles the application, and starts it.
func run(ctx context.Context) error {
	loadedConfig, err := bot.LoadEnviron(os.Environ())
	if err != nil {
		return err
	}

	err = configureLogging(loadedConfig.LogLevel, os.Stderr)
	if err != nil {
		return err
	}
	awsConfig, err := loadAWSConfig(ctx, loadedConfig.AWSRegion)
	if err != nil {
		return err
	}

	dynamoClient := newDynamoClient(awsConfig, loadedConfig.AWSEndpointURL)
	queueClient := queue.NewClient(newSQSClient(awsConfig, loadedConfig.AWSEndpointURL))
	telegramClient := newTelegramClient(loadedConfig)
	messageClient := dynamodb.NewMessageClient(dynamoClient)
	chatClient := dynamodb.NewChatClient(dynamoClient)
	scaleUpper := dynamodb.NewScaleUpper(dynamoClient, loadedConfig.MessageTable, loadedConfig.ScaleUpReadCapacity)
	actions := bot.NewService(
		chatClient,
		loadedConfig.ChatIDTable,
		messageClient,
		loadedConfig.MessageTable,
		telegramClient,
		time.Now,
		loadedConfig.EventQueueURL,
		queueClient,
	)
	queueWorker, err := newQueueWorker(loadedConfig.EventQueueURL, queueClient, actions)
	if err != nil {
		return err
	}
	readiness := &atomic.Bool{}
	httpServer, err := newHTTPServer(loadedConfig, readiness, chatClient, messageClient, queueClient, telegramClient, scaleUpper)
	if err != nil {
		return err
	}
	metricsServer := newMetricsServer(loadedConfig)
	application := bot.NewApp(
		bot.NewHTTPServer("HTTP", loadedConfig.ServerAddress, httpServer),
		bot.NewHTTPServer("metrics", loadedConfig.MetricsServerAddress, metricsServer),
		queueWorker,
		bot.AppOptions{
			Readiness:       readiness,
			ReadinessDrain:  loadedConfig.ReadinessDrain,
			ShutdownTimeout: loadedConfig.ShutdownTimeout,
		},
	)

	return application.Run(ctx)
}

// loadAWSConfig loads the AWS SDK config for the requested region.
func loadAWSConfig(ctx context.Context, region string) (awscore.Config, error) {
	awsConfig, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithRegion(region))
	if err != nil {
		return awscore.Config{}, fmt.Errorf("load AWS config: %w", err)
	}

	return awsConfig, nil
}

// newDynamoClient builds the DynamoDB client.
func newDynamoClient(awsConfig awscore.Config, endpointURL string) *awsdynamodb.Client {
	options := make([]func(*awsdynamodb.Options), 0, 1)
	if endpointURL != "" {
		options = append(options, func(clientOptions *awsdynamodb.Options) {
			clientOptions.BaseEndpoint = awscore.String(endpointURL)
		})
	}

	return awsdynamodb.NewFromConfig(awsConfig, options...)
}

// newSQSClient builds the SQS client.
func newSQSClient(awsConfig awscore.Config, endpointURL string) *awssqs.Client {
	options := make([]func(*awssqs.Options), 0, 1)
	if endpointURL != "" {
		options = append(options, func(clientOptions *awssqs.Options) {
			clientOptions.BaseEndpoint = awscore.String(endpointURL)
		})
	}

	return awssqs.NewFromConfig(awsConfig, options...)
}

// newTelegramClient builds the Telegram API client.
func newTelegramClient(loadedConfig bot.Config) bot.Client {
	return bot.NewClient(
		loadedConfig.TelegramBotToken,
		bot.WithBaseURL(loadedConfig.TelegramAPIBaseURL),
		bot.WithHTTPClient(&http.Client{Timeout: loadedConfig.HTTPTimeout}),
	)
}

// newHTTPServer builds the production HTTP server.
func newHTTPServer(
	loadedConfig bot.Config,
	readiness *atomic.Bool,
	chats dynamodb.ChatClient,
	messages dynamodb.MessageClient,
	sender interface {
		SendMessage(ctx context.Context, request queue.SendMessageRequest) error
	},
	messenger bot.Client,
	scaleUpper dynamodb.ScaleUpper,
) (*http.Server, error) {
	dependencies := httpserver.Dependencies{
		ChatTable:            loadedConfig.ChatIDTable,
		MessageTable:         loadedConfig.MessageTable,
		Chats:                chats,
		Messages:             messages,
		Enqueuer:             queue.NewProducer(loadedConfig.EventQueueURL, sender),
		Messenger:            messenger,
		ScaleUpper:           scaleUpper,
		Now:                  time.Now,
		WebhookSecretToken:   loadedConfig.WebhookSecretToken,
		SchedulerSecretToken: loadedConfig.SchedulerSecretToken,
		Readiness:            readiness,
	}

	return httpserver.NewServer(
		loadedConfig.ServerAddress,
		loadedConfig.HTTPTimeout,
		loadedConfig.Stage,
		dependencies,
	)
}

// newMetricsServer builds the dedicated Prometheus metrics server.
func newMetricsServer(loadedConfig bot.Config) *http.Server {
	return &http.Server{
		Addr:              loadedConfig.MetricsServerAddress,
		Handler:           promhttp.Handler(),
		ReadHeaderTimeout: loadedConfig.HTTPTimeout,
		ReadTimeout:       loadedConfig.HTTPTimeout,
		WriteTimeout:      loadedConfig.HTTPTimeout,
		IdleTimeout:       loadedConfig.HTTPTimeout,
	}
}

// newQueueWorker builds the production queue worker.
func newQueueWorker(queueURL string, queueClient interface {
	ReceiveMessage(ctx context.Context, request queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error)
	Delete(ctx context.Context, request queue.DeleteMessageRequest) error
}, actions bot.Service) (interface {
	Run(ctx context.Context) error
}, error) {
	return bot.NewPollingWorker(
		queueURL,
		queueClient,
		queueClient,
		bot.Handlers{
			JungHelp:       actions.JungHelp,
			TopTen:         actions.TopTen,
			TopDiver:       actions.TopDiver,
			AllJung:        actions.AllJung,
			OffFromWork:    actions.OffFromWork,
			EnableAllJung:  actions.EnableAllJung,
			DisableAllJung: actions.DisableAllJung,
			SetOffWorkTime: actions.SetOffWorkTime,
			OnOffFromWork:  actions.OnOffFromWork,
		},
	)
}

// configureLogging installs the process-wide slog handler from LOG_LEVEL.
// For example, "debug" enables debug logs on the default logger.
func configureLogging(level string, output io.Writer) error {
	var slogLevel slog.Level

	normalised := strings.ToLower(strings.TrimSpace(level))
	if normalised == "" {
		normalised = "info"
	}

	switch normalised {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return fmt.Errorf("unsupported log level %q", level)
	}

	handler := slog.NewTextHandler(output, &slog.HandlerOptions{Level: slogLevel})
	slog.SetDefault(slog.New(handler))

	return nil
}
