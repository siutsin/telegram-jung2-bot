package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ActionResult represents the outcome of processing a Telegram webhook payload.
// It is intentionally small while the service grows into package-specific
// handlers.
type ActionResult struct {
	StatusCode   int
	ChatID       int64
	ResponseText string
}

type webhookPayload struct {
	Message *struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

// ProcessWebhook performs the first Go-only validation pass for a Telegram
// webhook payload. The implementation is deliberately small while the full
// command router and persistence layers are implemented.
func ProcessWebhook(payload string) (ActionResult, error) {
	if strings.TrimSpace(payload) == "" {
		result := ActionResult{StatusCode: 400}
		return result, fmt.Errorf("webhook payload is empty")
	}

	var update webhookPayload
	if err := json.Unmarshal([]byte(payload), &update); err != nil {
		result := ActionResult{StatusCode: 400}
		return result, fmt.Errorf("decode webhook payload: %w", err)
	}

	if update.Message == nil {
		result := ActionResult{StatusCode: 202}
		return result, nil
	}

	return ActionResult{
		StatusCode:   200,
		ChatID:       update.Message.Chat.ID,
		ResponseText: "Go webhook processing scaffold",
	}, nil
}
