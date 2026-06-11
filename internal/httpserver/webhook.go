package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/command"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

// handleWebhook processes a Telegram webhook payload.
func handleWebhook(ctx context.Context, payload []byte, dependencies Dependencies) response {
	telegramMessage, result, ok := parseGroupMessage(payload)
	if !ok {
		return result
	}
	if saveResult, saved := saveWebhookState(ctx, *telegramMessage, currentTime(dependencies), dependencies); !saved {
		return saveResult
	}

	return enqueueWebhookCommands(ctx, *telegramMessage, dependencies)
}

// parseGroupMessage parses a Telegram webhook and keeps only group messages.
// For example, a private-chat webhook is filtered out with a 204 response.
func parseGroupMessage(payload []byte) (*telegram.Message, response, bool) {
	update, err := telegram.ParseUpdate(payload)
	if err != nil {
		slog.Warn("decode Telegram update", "err", err)
		return nil, response{statusCode: 500, message: "decode Telegram update"}, false
	}
	if update.Message == nil {
		return nil, response{statusCode: 204, message: "edited_message or non-group"}, false
	}
	if update.Message.Chat.Type == "" {
		return nil, response{statusCode: 500, message: "decode Telegram update"}, false
	}
	if !isGroupChat(update.Message.Chat.Type) {
		return nil, response{statusCode: 204, message: "edited_message or non-group"}, false
	}

	return update.Message, response{}, true
}

// saveWebhookState persists the message and chat records for a webhook update.
// For example, one Telegram message becomes one saved message row plus one saved
// chat metadata row.
func saveWebhookState(ctx context.Context, telegramMessage telegram.Message, now time.Time, dependencies Dependencies) (response, bool) {
	err := saveWebhookMessage(ctx, telegramMessage, now, dependencies)
	if err != nil {
		slog.Error("save webhook message", "err", err)
		return response{statusCode: 500, message: "save message"}, false
	}
	err = saveWebhookChat(ctx, telegramMessage, now, dependencies)
	if err != nil {
		slog.Error("save webhook chat", "err", err)
		return response{statusCode: 500, message: "save chat"}, false
	}

	return response{}, true
}

// saveWebhookMessage persists a Telegram message row.
// For example, a webhook message becomes message.FromTelegram(...) before save.
func saveWebhookMessage(ctx context.Context, telegramMessage telegram.Message, now time.Time, dependencies Dependencies) error {
	storedMessage := message.FromTelegram(telegramMessage, now)
	return dependencies.Messages.Save(ctx, dependencies.MessageTable, storedMessage)
}

// saveWebhookChat persists Telegram chat metadata.
// For example, a webhook message becomes chat.FromTelegram(...) before save.
func saveWebhookChat(ctx context.Context, telegramMessage telegram.Message, now time.Time, dependencies Dependencies) error {
	storedChat := chat.FromTelegram(telegramMessage, now)
	return dependencies.Chats.Save(ctx, dependencies.ChatTable, storedChat)
}

// enqueueWebhookCommands converts and enqueues supported Telegram commands.
// For example, "/topTen /allJung" is parsed and enqueued in the contract order.
func enqueueWebhookCommands(ctx context.Context, telegramMessage telegram.Message, dependencies Dependencies) response {
	for _, parsedCommand := range parseCommands(telegramMessage) {
		result, ok := enqueueWebhookCommand(ctx, telegramMessage, parsedCommand, dependencies)
		if !ok {
			return result
		}
	}

	return response{statusCode: 200}
}

// enqueueWebhookCommand converts one parsed command into queue work.
// For example, topTen becomes one queue action with chatId and chatTitle
// attributes.
func enqueueWebhookCommand(ctx context.Context, telegramMessage telegram.Message, parsedCommand command.Command, dependencies Dependencies) (response, bool) {
	action, err := command.ActionFor(parsedCommand, command.ChatContext{
		ChatID:    telegramMessage.Chat.ID,
		ChatTitle: telegramMessage.Chat.Title,
		UserID:    userID(telegramMessage.From),
	})
	if err == nil {
		err = dependencies.Enqueuer.Enqueue(ctx, action)
		if err != nil {
			slog.Error("enqueue webhook command", "action", action.Name, "err", err)
			return response{statusCode: 500, message: "enqueue command"}, false
		}
		return response{}, true
	}
	if shouldIgnoreCommandError(parsedCommand) {
		return response{}, true
	}
	err = sendInvalidSetOffReply(ctx, telegramMessage, dependencies)
	if err != nil {
		slog.Error("reply invalid set-off command", "err", err)
		return response{statusCode: 500, message: "reply invalid command"}, false
	}

	return response{}, true
}

// shouldIgnoreCommandError reports whether a command error should be skipped.
func shouldIgnoreCommandError(parsedCommand command.Command) bool {
	return !command.IsSetOffWorkTime(parsedCommand)
}

// sendInvalidSetOffReply sends the contract reply for invalid off-work input.
func sendInvalidSetOffReply(ctx context.Context, telegramMessage telegram.Message, dependencies Dependencies) error {
	if dependencies.Messenger == nil {
		return fmt.Errorf("messenger is required")
	}

	return dependencies.Messenger.SendMessage(
		ctx,
		telegramMessage.Chat.ID,
		schedule.InvalidSetOffFromWorkTimeUTCMessage(telegramMessage.Chat.Title),
	)
}

// currentTime returns the injected time or time.Now.
func currentTime(dependencies Dependencies) time.Time {
	if dependencies.Now == nil {
		return time.Now()
	}

	return dependencies.Now()
}

// userID returns the Telegram user ID or zero.
// For example, a nil user becomes 0.
func userID(user *telegram.User) int64 {
	if user == nil {
		return 0
	}

	return user.ID
}

// parseCommands extracts supported bot commands from a message.
// For example, a first entity of type bot_command allows command.ParseAll to run
// over message.Text.
func parseCommands(telegramMessage telegram.Message) []command.Command {
	if len(telegramMessage.Entities) == 0 || telegramMessage.Entities[0].Type != "bot_command" {
		return nil
	}

	return command.ParseAll(telegramMessage.Text)
}

// isGroupChat reports whether a Telegram chat type is a group conversation.
func isGroupChat(chatType string) bool {
	return chatType == "group" || chatType == "supergroup"
}
