// Package bootstrap owns process startup composition outside main.
package bootstrap

import (
	"context"

	"github.com/siutsin/telegram-jung2-bot/internal/app"
	"github.com/siutsin/telegram-jung2-bot/internal/config"
)

// Options configures startup composition.
type Options struct {
	AppOptions app.Options
}

// New loads config and constructs the application.
func New(ctx context.Context, environ []string, options Options) (*app.App, error) {
	loaded, err := config.LoadEnviron(environ)
	if err != nil {
		return nil, err
	}

	return app.New(ctx, loaded, options.AppOptions)
}

// Run loads config, constructs the application, and starts it.
func Run(ctx context.Context, environ []string, options Options) error {
	application, err := New(ctx, environ, options)
	if err != nil {
		return err
	}

	return application.Run(ctx)
}
