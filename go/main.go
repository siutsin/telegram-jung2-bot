package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/siutsin/telegram-jung2-bot/go/core"
)

func main() {
	// Test the FFI binding with a sample payload
	payload := `{"message":{"chat":{"id":123},"text":"test"}}`

	result, err := core.ProcessWebhook(payload)
	if err != nil {
		slog.Error("ProcessWebhook failed", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Status Code: %d\n", result.StatusCode)
	fmt.Printf("Chat ID: %d\n", result.ChatID)
	fmt.Printf("Response Text: %s\n", result.ResponseText)
}
