package main

import (
	"context"
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
	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
	contractdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/service"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

const stopMessage = "telegram-jung2-bot stopped"

// main starts the bot process.
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	loadedConfig, err := config.LoadEnviron(os.Environ())
	if err != nil {
		slog.Error(stopMessage, "error", err)
		os.Exit(1)
	}

	awsConfig, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithRegion(loadedConfig.AWSRegion))
	if err != nil {
		slog.Error(stopMessage, "error", err)
		os.Exit(1)
	}

	dynamoOptions := make([]func(*awsdynamodb.Options), 0, 1)
	sqsOptions := make([]func(*awssqs.Options), 0, 1)
	if loadedConfig.AWSEndpointURL != "" {
		dynamoOptions = append(dynamoOptions, func(options *awsdynamodb.Options) {
			options.BaseEndpoint = awscore.String(loadedConfig.AWSEndpointURL)
		})
		sqsOptions = append(sqsOptions, func(options *awssqs.Options) {
			options.BaseEndpoint = awscore.String(loadedConfig.AWSEndpointURL)
		})
	}

	dynamoClient := awsdynamodb.NewFromConfig(awsConfig, dynamoOptions...)
	queueClient := awssqs.NewFromConfig(awsConfig, sqsOptions...)
	telegramClient := telegram.NewClient(
		loadedConfig.TelegramBotToken,
		telegram.WithBaseURL(loadedConfig.TelegramAPIBaseURL),
		telegram.WithHTTPClient(&http.Client{Timeout: loadedConfig.HTTPTimeout}),
	)
	messageClient := contractdynamodb.MessageClient{Dynamo: dynamoClient}
	chatClient := contractdynamodb.ChatClient{Dynamo: dynamoClient}
	messageRepository := message.Repository{TableName: loadedConfig.MessageTable, Client: messageClient}
	chatRepository := chat.Repository{TableName: loadedConfig.ChatIDTable, Client: chatClient}
	queueAdapter := queue.Client{Queue: queueClient}
	actions := service.Service{
		ChatStore:         chatClient,
		ChatTable:         loadedConfig.ChatIDTable,
		MessageRepository: messageRepository,
		Messenger:         telegramClient,
		Now:               time.Now,
		QueueURL:          loadedConfig.EventQueueURL,
		Sender:            queueAdapter,
	}

	application, err := app.New(loadedConfig, app.Dependencies{
		Chats:      chatRepository,
		Messages:   messageRepository,
		Sender:     queueAdapter,
		Receiver:   queueAdapter,
		Deleter:    queueAdapter,
		Messenger:  telegramClient,
		ScaleUpper: contractdynamodb.ScaleUpper{Dynamo: dynamoClient, DesiredRead: loadedConfig.ScaleUpReadCapacity, TableName: loadedConfig.MessageTable},
		Handlers: worker.Handlers{
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
		Now: time.Now,
	}, app.Options{})
	if err != nil {
		slog.Error(stopMessage, "error", err)
		os.Exit(1)
	}

	err = application.Run(ctx)
	if err != nil {
		slog.Error(stopMessage, "error", err)
		os.Exit(1)
	}
}
