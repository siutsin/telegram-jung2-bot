package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	bot "github.com/siutsin/telegram-jung2-bot/internal"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

// handleWebhook processes a Telegram webhook payload.
func handleWebhook(ctx context.Context, payload []byte, dependencies Dependencies) response {
	update, err := bot.ParseUpdate(payload)
	if err == nil && update.MyChatMember != nil {
		return handleMyChatMember(ctx, update.MyChatMember, dependencies)
	}
	parsed, result, ok := parseGroupMessage(payload)
	if !ok {
		return result
	}
	if result, saved := enqueueWebhookSave(ctx, parsed.UpdateID, *parsed.Message, dependencies); !saved {
		return result
	}

	return enqueueWebhookCommands(ctx, *parsed.Message, dependencies)
}

// handleMyChatMember deletes the chat row when the bot leaves or is kicked from a group.
// For example, a left status in a supergroup deletes chat 123 and returns 200.
func handleMyChatMember(ctx context.Context, memberUpdate *bot.ChatMemberUpdated, dependencies Dependencies) response {
	if !isRemovedChatMemberStatus(memberUpdate.NewChatMember.Status) || !isGroupChat(memberUpdate.Chat.Type) {
		return response{statusCode: 204, message: "edited_message or non-group"}
	}

	err := dependencies.ChatDeleter.DeleteChat(ctx, dependencies.ChatTable, memberUpdate.Chat.ID)
	if err != nil {
		slog.Error("delete chat on bot removal", "chatId", memberUpdate.Chat.ID, "oldStatus", memberUpdate.OldChatMember.Status, "newStatus", memberUpdate.NewChatMember.Status, "err", err)
		return response{statusCode: 200}
	}

	slog.Info("deleted chat on bot removal", "chatId", memberUpdate.Chat.ID, "oldStatus", memberUpdate.OldChatMember.Status, "newStatus", memberUpdate.NewChatMember.Status)
	return response{statusCode: 200}
}

// isRemovedChatMemberStatus reports whether the bot left the chat.
// For example, "left" and "kicked" return true while "member" returns false.
func isRemovedChatMemberStatus(status string) bool {
	return status == "left" || status == "kicked"
}

// parseGroupMessage parses a Telegram webhook and keeps only group messages.
// For example, a private-chat webhook is filtered out with a 204 response.
func parseGroupMessage(payload []byte) (*bot.Update, response, bool) {
	update, err := bot.ParseUpdate(payload)
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

	return &update, response{}, true
}

// enqueueWebhookSave sends one idempotent message-save action to the FIFO queue.
// For example, chat 42 and update 7 get the deduplication ID "42:7".
func enqueueWebhookSave(ctx context.Context, updateID int64, telegramMessage bot.TelegramMessage, dependencies Dependencies) (response, bool) {
	action := queue.Action{
		Name:                   queue.ActionSaveMessage,
		Body:                   queue.BodySaveMessage,
		MessageGroupID:         strconv.FormatInt(telegramMessage.Chat.ID, 10),
		MessageDeduplicationID: strconv.FormatInt(telegramMessage.Chat.ID, 10) + ":" + strconv.FormatInt(updateID, 10),
		Attributes: map[string]string{
			"action":    queue.ActionSaveMessage,
			"chatId":    strconv.FormatInt(telegramMessage.Chat.ID, 10),
			"chatTitle": telegramMessage.Chat.Title,
			"messageId": strconv.FormatInt(telegramMessage.MessageID, 10),
			"date":      strconv.FormatInt(telegramMessage.Date, 10),
		},
	}
	if telegramMessage.From != nil {
		action.Attributes["userId"] = strconv.FormatInt(telegramMessage.From.ID, 10)
		action.Attributes["username"] = telegramMessage.From.UserName
		action.Attributes["firstName"] = telegramMessage.From.FirstName
		action.Attributes["lastName"] = telegramMessage.From.LastName
	}

	err := dependencies.MessageEnqueuer.Enqueue(ctx, action)
	if err != nil {
		slog.Error("enqueue webhook message save", "err", err)
		return response{statusCode: 500, message: "enqueue message save"}, false
	}

	return response{}, true
}

// enqueueWebhookCommands converts and enqueues supported Telegram commands.
// For example, "/topTen /allJung" is parsed and enqueued in the contract order.
func enqueueWebhookCommands(ctx context.Context, telegramMessage bot.TelegramMessage, dependencies Dependencies) response {
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
func enqueueWebhookCommand(ctx context.Context, telegramMessage bot.TelegramMessage, parsedCommand bot.Command, dependencies Dependencies) (response, bool) {
	action, err := bot.ActionFor(parsedCommand, bot.ChatContext{
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
		if dependencies.Metrics != nil {
			dependencies.Metrics.RecordWebhookCommand(action.Name)
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
func shouldIgnoreCommandError(parsedCommand bot.Command) bool {
	return !bot.IsSetOffWorkTime(parsedCommand)
}

// sendInvalidSetOffReply sends the contract reply for invalid off-work input.
func sendInvalidSetOffReply(ctx context.Context, telegramMessage bot.TelegramMessage, dependencies Dependencies) error {
	if dependencies.Messenger == nil {
		return fmt.Errorf("messenger is required")
	}

	return dependencies.Messenger.SendMessage(
		ctx,
		telegramMessage.Chat.ID,
		bot.InvalidSetOffFromWorkTimeUTCMessage(telegramMessage.Chat.Title),
	)
}

// userID returns the Telegram user ID or zero.
// For example, a nil user becomes 0.
func userID(user *bot.User) int64 {
	if user == nil {
		return 0
	}

	return user.ID
}

// parseCommands extracts supported bot commands from a bot.
// For example, a first entity of type bot_command allows bot.ParseAll to run
// over bot.Text.
func parseCommands(telegramMessage bot.TelegramMessage) []bot.Command {
	if len(telegramMessage.Entities) == 0 || telegramMessage.Entities[0].Type != "bot_command" {
		return nil
	}

	return bot.ParseAll(telegramMessage.Text)
}

// isGroupChat reports whether a Telegram chat type is a group conversation.
func isGroupChat(chatType string) bool {
	return chatType == "group" || chatType == "supergroup"
}
