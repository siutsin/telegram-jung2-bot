package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/siutsin/telegram-jung2-bot/internal/bootstrap"
)

// main starts the bot process.
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := bootstrap.Run(ctx, os.Environ(), bootstrap.Options{}); err != nil {
		slog.Error("telegram-jung2-bot stopped", "error", err)
		os.Exit(1)
	}
}
