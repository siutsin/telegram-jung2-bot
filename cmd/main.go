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
	app, err := newApp(ctx, loadedConfig)
	if err != nil {
		return err
	}

	return app.run(ctx)
}

// app keeps runtime dependencies together so a test can replace one at a time.
type app struct {
	config     bot.Config
	readiness  *atomic.Bool
	metrics    *bot.Metrics
	queue      queueTransporter
	messages   messageKeeper
	chats      chatMaintainer
	messenger  telegramMessenger
	scaleUpper scaleUpper
}

type queueTransporter interface {
	SendMessage(ctx context.Context, request queue.SendMessageRequest) error
	ReceiveMessage(ctx context.Context, request queue.ReceiveMessageRequest) (queue.ReceiveMessageResponse, error)
	Delete(ctx context.Context, request queue.DeleteMessageRequest) error
}

type messageKeeper interface {
	Save(ctx context.Context, tableName string, row bot.StoredMessage) error
	QueryByChat(ctx context.Context, tableName string, chatID int64, since time.Time) ([]bot.StoredMessage, error)
}

type chatMaintainer interface {
	DueChatIDs(ctx context.Context, tableName string, timestamp time.Time) ([]int64, error)
	Get(ctx context.Context, tableName string, chatID int64) (bot.ChatSetting, bool, error)
	Save(ctx context.Context, tableName string, settings bot.ChatSetting) error
	SaveStatistics(ctx context.Context, tableName string, chatID int64, userCount int, messageCount int, now time.Time) error
	Update(ctx context.Context, update bot.UpdateExpression) error
}

type telegramMessenger interface {
	IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageWithOptions(ctx context.Context, chatID int64, text string, options bot.SendMessageOptions) error
}

type scaleUpper interface {
	ScaleUp(ctx context.Context) error
}

// newApp builds production dependencies once at the process boundary.
func newApp(ctx context.Context, loadedConfig bot.Config) (app, error) {
	awsConfig, err := loadAWSConfig(ctx, loadedConfig.AWSRegion)
	if err != nil {
		return app{}, err
	}

	readiness := &atomic.Bool{}
	metrics := bot.NewMetrics(readiness)
	dynamoClient := newDynamoClient(awsConfig, loadedConfig.AWSEndpointURL)
	queueClient := queue.NewClient(newSQSClient(awsConfig, loadedConfig.AWSEndpointURL), metrics)

	return app{
		config:     loadedConfig,
		readiness:  readiness,
		metrics:    metrics,
		queue:      queueClient,
		messages:   dynamodb.NewMessageClient(dynamoClient, metrics),
		chats:      dynamodb.NewChatClient(dynamoClient, metrics),
		messenger:  newTelegramClient(loadedConfig, metrics),
		scaleUpper: dynamodb.NewScaleUpper(dynamoClient, loadedConfig.MessageTable, loadedConfig.ScaleUpReadCapacity, metrics),
	}, nil
}

// run starts the assembled application after tests have swapped dependencies.
func (app app) run(ctx context.Context) error {
	actions := bot.NewService(
		app.chats,
		app.config.ChatIDTable,
		app.messages,
		app.config.MessageTable,
		app.messenger,
		time.Now,
		app.config.EventQueueURL,
		app.queue,
		app.metrics,
	)
	queueWorker, err := newQueueWorker(app, actions)
	if err != nil {
		return err
	}
	httpServer, err := newHTTPServer(app)
	if err != nil {
		return err
	}
	metricsServer := newMetricsServer(app)
	runtime := bot.NewApp(
		bot.NewHTTPServer("HTTP", app.config.ServerAddress, httpServer),
		bot.NewHTTPServer("metrics", app.config.MetricsServerAddress, metricsServer),
		queueWorker,
		bot.AppOptions{
			Readiness:       app.readiness,
			ReadinessDrain:  app.config.ReadinessDrain,
			ShutdownTimeout: app.config.ShutdownTimeout,
		},
	)

	return runtime.Run(ctx)
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
func newTelegramClient(loadedConfig bot.Config, metrics *bot.Metrics) bot.Client {
	return bot.NewClient(
		loadedConfig.TelegramBotToken,
		bot.WithBaseURL(loadedConfig.TelegramAPIBaseURL),
		bot.WithHTTPClient(&http.Client{Timeout: loadedConfig.HTTPTimeout}),
		bot.WithDependencyObserver(metrics),
	)
}

// newHTTPServer keeps HTTP wiring in the app so tests can swap one dependency.
func newHTTPServer(app app) (*http.Server, error) {
	dependencies := httpserver.Dependencies{
		ChatTable:            app.config.ChatIDTable,
		MessageTable:         app.config.MessageTable,
		Chats:                app.chats,
		Messages:             app.messages,
		Enqueuer:             queue.NewProducer(app.config.EventQueueURL, app.queue),
		Messenger:            app.messenger,
		ScaleUpper:           app.scaleUpper,
		Now:                  time.Now,
		WebhookSecretToken:   app.config.WebhookSecretToken,
		SchedulerSecretToken: app.config.SchedulerSecretToken,
		Readiness:            app.readiness,
		Metrics:              app.metrics,
	}

	return httpserver.NewServer(
		app.config.ServerAddress,
		app.config.HTTPTimeout,
		app.config.Stage,
		dependencies,
	)
}

// newMetricsServer builds the dedicated Prometheus metrics server.
func newMetricsServer(app app) *http.Server {
	return &http.Server{
		Addr:              app.config.MetricsServerAddress,
		Handler:           app.metrics.Handler(),
		ReadHeaderTimeout: app.config.HTTPTimeout,
		ReadTimeout:       app.config.HTTPTimeout,
		WriteTimeout:      app.config.HTTPTimeout,
		IdleTimeout:       app.config.HTTPTimeout,
	}
}

// newQueueWorker builds the production queue worker.
func newQueueWorker(app app, actions bot.Service) (interface {
	Run(ctx context.Context) error
}, error) {
	return bot.NewPollingWorker(
		app.config.EventQueueURL,
		app.queue,
		app.queue,
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
		}, app.metrics,
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
