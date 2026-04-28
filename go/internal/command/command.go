// Package command parses Telegram command text and maps commands to actions.
package command

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/siutsin/telegram-jung2-bot/go/internal/queue"
)

var commandPattern = regexp.MustCompile(`(?i)/(jungHelp|topTen|topDiver|allJung|enableAllJung|disableAllJung|setOffFromWorkTimeUTC)\b`)

// Command names supported by the bot.
const (
	JungHelp              = "jungHelp"
	TopTen                = "topTen"
	TopDiver              = "topDiver"
	AllJung               = "allJung"
	EnableAllJung         = "enableAllJung"
	DisableAllJung        = "disableAllJung"
	SetOffFromWorkTimeUTC = "setOffFromWorkTimeUTC"
)

// Command is a parsed Telegram command.
type Command struct {
	Name string
	Args string
}

// ChatContext contains the chat and user fields needed to create actions.
type ChatContext struct {
	ChatID    int64  `json:"chatId"`
	ChatTitle string `json:"chatTitle,omitempty"`
	UserID    int64  `json:"userId,omitempty"`
}

// Parse returns the first supported command found in text.
func Parse(text string) (Command, bool) {
	match := commandPattern.FindStringSubmatchIndex(text)
	if match == nil {
		return Command{}, false
	}

	name := canonicalName(text[match[2]:match[3]])
	args := strings.TrimSpace(text[match[1]:])

	return Command{Name: name, Args: args}, true
}

// ActionFor converts a command and chat context into a stable queue action.
func ActionFor(command Command, chat ChatContext) (queue.Action, error) {
	name, err := actionName(command.Name)
	if err != nil {
		return queue.Action{}, err
	}

	payload, err := json.Marshal(struct {
		ChatContext
		Args string `json:"args,omitempty"`
	}{
		ChatContext: chat,
		Args:        command.Args,
	})
	if err != nil {
		return queue.Action{}, fmt.Errorf("encode command action payload: %w", err)
	}

	return queue.Action{
		Name:    name,
		Payload: payload,
	}, nil
}

func canonicalName(name string) string {
	switch strings.ToLower(name) {
	case strings.ToLower(JungHelp):
		return JungHelp
	case strings.ToLower(TopTen):
		return TopTen
	case strings.ToLower(TopDiver):
		return TopDiver
	case strings.ToLower(AllJung):
		return AllJung
	case strings.ToLower(EnableAllJung):
		return EnableAllJung
	case strings.ToLower(DisableAllJung):
		return DisableAllJung
	case strings.ToLower(SetOffFromWorkTimeUTC):
		return SetOffFromWorkTimeUTC
	default:
		return name
	}
}

func actionName(commandName string) (string, error) {
	switch commandName {
	case JungHelp:
		return queue.ActionJungHelp, nil
	case TopTen:
		return queue.ActionTopTen, nil
	case TopDiver:
		return queue.ActionTopDiver, nil
	case AllJung:
		return queue.ActionAllJung, nil
	case EnableAllJung:
		return queue.ActionEnableAllJung, nil
	case DisableAllJung:
		return queue.ActionDisableAllJung, nil
	case SetOffFromWorkTimeUTC:
		return queue.ActionSetOffWorkTime, nil
	default:
		return "", fmt.Errorf("unsupported command %q", commandName)
	}
}
