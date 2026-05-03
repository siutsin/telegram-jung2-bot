package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/siutsin/telegram-jung2-bot/internal/app"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
	"github.com/siutsin/telegram-jung2-bot/internal/runtime"
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

	components, err := runtime.NewComponents(ctx, loadedConfig)
	if err != nil {
		slog.Error(stopMessage, "error", err)
		os.Exit(1)
	}

	application, err := app.New(loadedConfig, app.Dependencies{
		Store:      components.Store,
		Sender:     components.Sender,
		Receiver:   components.Receiver,
		Deleter:    components.Deleter,
		Messenger:  components.Messenger,
		ScaleUpper: components.ScaleUpper,
		Handlers:   components.Handlers,
		Now:        components.Now,
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
