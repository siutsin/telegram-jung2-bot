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
	"syscall"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/siutsin/telegram-jung2-bot/internal/app"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
	"github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/httpserver"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/service"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
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
	loadedConfig, err := config.LoadEnviron(os.Environ())
	if err != nil {
		return err
	}

	err = configureLogging(loadedConfig.LogLevel, os.Stderr)
	if err != nil {
		return err
	}
	warnIfWebhookSecretMissing(loadedConfig)

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
	actions := service.New(
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
	httpServer, err := newHTTPServer(loadedConfig, chatClient, messageClient, queueClient, telegramClient, scaleUpper)
	if err != nil {
		return err
	}
	application := app.New(
		httpServer,
		queueWorker,
		app.Options{ShutdownTimeout: loadedConfig.ShutdownTimeout},
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
func newTelegramClient(loadedConfig config.Config) telegram.Client {
	return telegram.NewClient(
		loadedConfig.TelegramBotToken,
		telegram.WithBaseURL(loadedConfig.TelegramAPIBaseURL),
		telegram.WithHTTPClient(&http.Client{Timeout: loadedConfig.HTTPTimeout}),
	)
}

// newHTTPServer builds the production HTTP server.
func newHTTPServer(
	loadedConfig config.Config,
	chats dynamodb.ChatClient,
	messages dynamodb.MessageClient,
	sender interface {
		SendMessage(ctx context.Context, request queue.SendMessageRequest) error
	},
	messenger telegram.Client,
	scaleUpper dynamodb.ScaleUpper,
) (*http.Server, error) {
	dependencies := httpserver.Dependencies{
		ChatTable:          loadedConfig.ChatIDTable,
		MessageTable:       loadedConfig.MessageTable,
		Chats:              chats,
		Messages:           messages,
		Enqueuer:           queue.NewProducer(loadedConfig.EventQueueURL, sender),
		Messenger:          messenger,
		ScaleUpper:         scaleUpper,
		Now:                time.Now,
		WebhookSecretToken: loadedConfig.WebhookSecretToken,
	}

	return httpserver.NewServer(
		loadedConfig.ServerAddress,
		loadedConfig.HTTPTimeout,
		loadedConfig.Stage,
		dependencies,
	)
}

// newQueueWorker builds the production queue worker.
func newQueueWorker(queueURL string, queueClient interface {
	ReceiveMessage(ctx context.Context, request queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error)
	Delete(ctx context.Context, request queue.DeleteMessageRequest) error
}, actions service.Service) (interface {
	Run(ctx context.Context) error
}, error) {
	return worker.NewPollingWorker(
		queueURL,
		queueClient,
		queueClient,
		worker.Handlers{
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
		slogLevel = slog.LevelInfo
		handler := slog.NewTextHandler(output, &slog.HandlerOptions{Level: slogLevel})
		slog.SetDefault(slog.New(handler))
		slog.Warn("unknown log level, defaulting to info", "level", level)

		return nil
	}

	handler := slog.NewTextHandler(output, &slog.HandlerOptions{Level: slogLevel})
	slog.SetDefault(slog.New(handler))

	return nil
}

// warnIfWebhookSecretMissing logs when non-dev stages run without webhook auth.
func warnIfWebhookSecretMissing(loadedConfig config.Config) {
	if strings.TrimSpace(loadedConfig.WebhookSecretToken) != "" {
		return
	}

	stage := strings.ToLower(strings.TrimSpace(loadedConfig.Stage))
	if stage == "dev" || stage == "test" || stage == "local" {
		return
	}

	slog.Warn("WEBHOOK_SECRET_TOKEN is not set; webhook and stage routes accept unsigned requests", "stage", loadedConfig.Stage)
}
