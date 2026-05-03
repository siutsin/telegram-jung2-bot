// Package command parses Telegram command text and maps commands to actions.
package command

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

var commandPattern = regexp.MustCompile(`(?i)/(jungHelp|topTen|topDiver|allJung|enableAllJung|disableAllJung|setOffFromWorkTimeUTC)\b`)
var offTimePattern = regexp.MustCompile(`^([0-1]\d|2[0-3])(00|15|30|45)$`)

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

	return commandFromMatch(text, match), true
}

// ParseAll returns supported commands in the same fixed order as the contract
// independent command checks.
func ParseAll(text string) []Command {
	commands := make([]Command, 0, len(commandCheckOrder))
	for _, name := range commandCheckOrder {
		pattern := regexp.MustCompile(`(?i)/` + regexp.QuoteMeta(name) + `\b`)
		match := pattern.FindStringSubmatchIndex(text)
		if match == nil {
			continue
		}
		commands = append(commands, commandFromMatch(text, match))
	}

	return commands
}

// commandFromMatch builds a command from a regexp match.
func commandFromMatch(text string, match []int) Command {
	name := canonicalName(strings.TrimPrefix(text[match[0]:match[1]], "/"))
	args := commandArgs(text[match[1]:])

	return Command{Name: name, Args: args}
}

// ActionFor converts a command and chat context into a stable queue action.
func ActionFor(command Command, chat ChatContext) (queue.Action, error) {
	action, err := baseAction(command.Name)
	if err != nil {
		return queue.Action{}, err
	}

	action.Attributes = map[string]string{
		"chatId": strconv.FormatInt(chat.ChatID, 10),
		"action": action.Name,
	}

	switch command.Name {
	case JungHelp:
		action.Attributes["chatTitle"] = chat.ChatTitle
	case EnableAllJung, DisableAllJung:
		action.Attributes["chatTitle"] = chat.ChatTitle
		action.Attributes["userId"] = strconv.FormatInt(chat.UserID, 10)
	case SetOffFromWorkTimeUTC:
		schedule, err := ParseSetOffFromWorkTimeArgs(command.Args)
		if err != nil {
			return queue.Action{}, err
		}
		action.Attributes["chatTitle"] = chat.ChatTitle
		action.Attributes["userId"] = strconv.FormatInt(chat.UserID, 10)
		action.Attributes["offTime"] = schedule.OffTime
		action.Attributes["workday"] = schedule.Workday
	}

	return action, nil
}

// SetOffFromWorkTimeArgs is the validated argument set for the admin command.
type SetOffFromWorkTimeArgs struct {
	OffTime string
	Workday string
}

// ParseSetOffFromWorkTimeArgs validates and normalises contract command args.
func ParseSetOffFromWorkTimeArgs(args string) (SetOffFromWorkTimeArgs, error) {
	parts := strings.Split(args, " ")
	if len(parts) != 2 {
		return SetOffFromWorkTimeArgs{}, fmt.Errorf("%s requires off time and workday list", SetOffFromWorkTimeUTC)
	}

	offTime := parts[0]
	if !offTimePattern.MatchString(offTime) {
		return SetOffFromWorkTimeArgs{}, fmt.Errorf("invalid off time %q", offTime)
	}

	normalisedWorkday, err := normaliseWorkdayList(parts[1])
	if err != nil {
		return SetOffFromWorkTimeArgs{}, err
	}

	return SetOffFromWorkTimeArgs{
		OffTime: offTime,
		Workday: normalisedWorkday,
	}, nil
}

// canonicalName restores the contract command casing.
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

// baseAction returns the base queue action for a command name.
func baseAction(commandName string) (queue.Action, error) {
	switch commandName {
	case JungHelp:
		return queue.Action{Name: queue.ActionJungHelp, Body: queue.BodyJungHelp}, nil
	case TopTen:
		return queue.Action{Name: queue.ActionTopTen, Body: queue.BodyTopTen}, nil
	case TopDiver:
		return queue.Action{Name: queue.ActionTopDiver, Body: queue.BodyTopDiver}, nil
	case AllJung:
		return queue.Action{Name: queue.ActionAllJung, Body: queue.BodyAllJung}, nil
	case EnableAllJung:
		return queue.Action{Name: queue.ActionEnableAllJung, Body: queue.BodyEnableAllJung}, nil
	case DisableAllJung:
		return queue.Action{Name: queue.ActionDisableAllJung, Body: queue.BodyDisableAllJung}, nil
	case SetOffFromWorkTimeUTC:
		return queue.Action{Name: queue.ActionSetOffWorkTime, Body: queue.BodySetOffWorkTime}, nil
	default:
		return queue.Action{}, fmt.Errorf("unsupported command %q", commandName)
	}
}

// normaliseWorkdayList removes duplicate day tokens in order.
func normaliseWorkdayList(raw string) (string, error) {
	parts := strings.Split(raw, ",")
	if len(parts) > 7 {
		return "", fmt.Errorf("invalid workday list %q", raw)
	}
	seen := make(map[string]bool, len(parts))
	normalised := make([]string, 0, len(parts))

	for _, part := range parts {
		_, err := workday.ParseList(part)
		if err != nil {
			return "", err
		}
		if !seen[part] {
			seen[part] = true
			normalised = append(normalised, part)
		}
	}

	return strings.Join(normalised, ","), nil
}

// commandArgs strips an optional bot mention before returning args.
func commandArgs(raw string) string {
	args := strings.TrimPrefix(raw, " ")
	if !strings.HasPrefix(args, "@") {
		return args
	}

	mentionEnd := strings.Index(args, " ")
	if mentionEnd < 0 {
		return ""
	}

	return strings.TrimPrefix(args[mentionEnd:], " ")
}

var commandCheckOrder = []string{
	JungHelp,
	TopTen,
	TopDiver,
	AllJung,
	EnableAllJung,
	DisableAllJung,
	SetOffFromWorkTimeUTC,
}
