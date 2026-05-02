package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/siutsin/telegram-jung2-bot/internal/app"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
)

func main() {
	if err := run(context.Background(), os.Environ()); err != nil {
		slog.Error("telegram-jung2-bot stopped", "error", err)
	}
}

func run(ctx context.Context, environ []string) error {
	loaded, err := config.Load(envMap(environ))
	if err != nil {
		return err
	}

	return app.Run(ctx, loaded)
}

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
