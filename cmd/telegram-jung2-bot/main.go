package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/siutsin/telegram-jung2-bot/internal/app"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
)

var runApp = app.Run

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
	loaded, err := config.Load(envMap(environ))
	if err != nil {
		return err
	}

	return runApp(ctx, loaded)
}

// envMap converts environment entries into a lookup map.
func envMap(environ []string) map[string]string {
	env := make(map[string]string, len(environ))
	for _, entry := range environ {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}

	return env
}
