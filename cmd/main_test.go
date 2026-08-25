package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bot "github.com/siutsin/telegram-jung2-bot/internal"
)

// TestNewMetricsServerUsesAppConfig proves tests can construct the app with selected dependencies.
func TestNewMetricsServerUsesAppConfig(t *testing.T) {
	testApp := app{
		config:  bot.Config{MetricsServerAddress: "127.0.0.1:9090", HTTPTimeout: time.Second},
		metrics: bot.NewMetrics(nil),
	}

	server := newMetricsServer(testApp)

	assert.Equal(t, testApp.config.MetricsServerAddress, server.Addr)
	assert.Equal(t, time.Second, server.ReadTimeout)
	assert.Equal(t, time.Second, server.ReadHeaderTimeout)
	assert.Equal(t, time.Second, server.WriteTimeout)
	assert.Equal(t, time.Second, server.IdleTimeout)
	require.NotNil(t, server.Handler)
}
