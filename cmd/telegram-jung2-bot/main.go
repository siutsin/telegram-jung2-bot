package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/siutsin/telegram-jung2-bot/internal/app"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
)

var loadConfig = config.LoadEnviron
var newApp = func(ctx context.Context, loaded config.Config) (*app.App, error) {
	return app.New(ctx, loaded, app.Options{})
}

// main starts the bot process.
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, os.Environ()); err != nil {
		slog.Error("telegram-jung2-bot stopped", "error", err)
		os.Exit(1)
	}
}

// run loads configuration and starts the app.
func run(ctx context.Context, environ []string) error {
	loaded, err := loadConfig(environ)
	if err != nil {
		return err
	}

	application, err := newApp(ctx, loaded)
	if err != nil {
		return err
	}

	return application.Run(ctx)
}
