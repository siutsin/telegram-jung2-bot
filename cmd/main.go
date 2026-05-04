package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

	awsConfig, err := loadAWSConfig(ctx, loadedConfig.AWSRegion)
	if err != nil {
		return err
	}

	dynamoClient := newDynamoClient(awsConfig, loadedConfig.AWSEndpointURL)
	queueClient := queue.Client{Queue: newSQSClient(awsConfig, loadedConfig.AWSEndpointURL)}
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
	chats httpserver.ChatSaver,
	messages httpserver.MessageSaver,
	sender queue.Sender,
	messenger httpserver.Messenger,
	scaleUpper httpserver.ScaleUpper,
) (*http.Server, error) {
	dependencies := httpserver.Dependencies{
		ChatTable:    loadedConfig.ChatIDTable,
		MessageTable: loadedConfig.MessageTable,
		Chats:        chats,
		Messages:     messages,
		Enqueuer:     queue.Producer{QueueURL: loadedConfig.EventQueueURL, Sender: sender},
		Messenger:    messenger,
		ScaleUpper:   scaleUpper,
		Now:          time.Now,
	}

	return httpserver.NewServer(
		loadedConfig.ServerAddress,
		loadedConfig.HTTPTimeout,
		loadedConfig.Stage,
		dependencies,
	)
}

// newQueueWorker builds the production queue worker.
func newQueueWorker(queueURL string, queueClient queue.Client, actions service.Service) (worker.PollingWorker, error) {
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
