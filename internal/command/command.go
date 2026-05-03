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

// SetOffFromWorkTimeUTC is the only command name needed outside this package.
const SetOffFromWorkTimeUTC = "setOffFromWorkTimeUTC"

const (
	jungHelp       = "jungHelp"
	topTen         = "topTen"
	topDiver       = "topDiver"
	allJung        = "allJung"
	enableAllJung  = "enableAllJung"
	disableAllJung = "disableAllJung"
)

var commandDefinitions = []commandDefinition{
	{
		Name:   jungHelp,
		Action: queue.Action{Name: queue.ActionJungHelp, Body: queue.BodyJungHelp},
		Regex:  regexp.MustCompile(`(?i)/` + regexp.QuoteMeta(jungHelp) + `\b`),
	},
	{
		Name:   topTen,
		Action: queue.Action{Name: queue.ActionTopTen, Body: queue.BodyTopTen},
		Regex:  regexp.MustCompile(`(?i)/` + regexp.QuoteMeta(topTen) + `\b`),
	},
	{
		Name:   topDiver,
		Action: queue.Action{Name: queue.ActionTopDiver, Body: queue.BodyTopDiver},
		Regex:  regexp.MustCompile(`(?i)/` + regexp.QuoteMeta(topDiver) + `\b`),
	},
	{
		Name:   allJung,
		Action: queue.Action{Name: queue.ActionAllJung, Body: queue.BodyAllJung},
		Regex:  regexp.MustCompile(`(?i)/` + regexp.QuoteMeta(allJung) + `\b`),
	},
	{
		Name:   enableAllJung,
		Action: queue.Action{Name: queue.ActionEnableAllJung, Body: queue.BodyEnableAllJung},
		Regex:  regexp.MustCompile(`(?i)/` + regexp.QuoteMeta(enableAllJung) + `\b`),
	},
	{
		Name:   disableAllJung,
		Action: queue.Action{Name: queue.ActionDisableAllJung, Body: queue.BodyDisableAllJung},
		Regex:  regexp.MustCompile(`(?i)/` + regexp.QuoteMeta(disableAllJung) + `\b`),
	},
	{
		Name:   SetOffFromWorkTimeUTC,
		Action: queue.Action{Name: queue.ActionSetOffWorkTime, Body: queue.BodySetOffWorkTime},
		Regex:  regexp.MustCompile(`(?i)/` + regexp.QuoteMeta(SetOffFromWorkTimeUTC) + `\b`),
	},
}

var offTimePattern = regexp.MustCompile(`^([0-1]\d|2[0-3])(00|15|30|45)$`)

// Command is a parsed Telegram command.
type Command struct {
	Name string
	Args string
}

type commandDefinition struct {
	Name   string
	Action queue.Action
	Regex  *regexp.Regexp
}

// ChatContext contains the chat and user fields needed to create actions.
type ChatContext struct {
	ChatID    int64  `json:"chatId"`
	ChatTitle string `json:"chatTitle,omitempty"`
	UserID    int64  `json:"userId,omitempty"`
}

// ParseAll returns supported commands in the same fixed order as the contract
// independent command checks.
// For example, "/allJung and /topTen" returns topTen before allJung.
func ParseAll(text string) []Command {
	commands := make([]Command, 0, len(commandDefinitions))
	for _, definition := range commandDefinitions {
		match := definition.Regex.FindStringSubmatchIndex(text)
		if match == nil {
			continue
		}
		commands = append(commands, commandFromMatch(text, match))
	}

	return commands
}

// commandFromMatch builds a command from a regexp match.
// For example, "/setOffFromWorkTimeUTC@jung2bot 1830 MON,TUE" becomes
// Command{Name: SetOffFromWorkTimeUTC, Args: "1830 MON,TUE"}.
func commandFromMatch(text string, match []int) Command {
	name := canonicalName(strings.TrimPrefix(text[match[0]:match[1]], "/"))
	args := commandArgs(text[match[1]:])

	return Command{Name: name, Args: args}
}

// ActionFor converts a command and chat context into a stable queue action.
// For example, Command{Name: jungHelp} becomes an action with chatId, action,
// and chatTitle attributes.
func ActionFor(command Command, chat ChatContext) (queue.Action, error) {
	var action queue.Action
	found := false
	for _, definition := range commandDefinitions {
		if strings.EqualFold(definition.Name, command.Name) {
			action = definition.Action
			found = true
			break
		}
	}
	if !found {
		return queue.Action{}, fmt.Errorf("unsupported command %q", command.Name)
	}

	action.Attributes = map[string]string{
		"chatId": strconv.FormatInt(chat.ChatID, 10),
		"action": action.Name,
	}

	switch command.Name {
	case jungHelp:
		action.Attributes["chatTitle"] = chat.ChatTitle
	case enableAllJung, disableAllJung:
		action.Attributes["chatTitle"] = chat.ChatTitle
		action.Attributes["userId"] = strconv.FormatInt(chat.UserID, 10)
	case SetOffFromWorkTimeUTC:
		offTime, workday, parseErr := parseSetOffFromWorkTimeArgs(command.Args)
		if parseErr != nil {
			return queue.Action{}, parseErr
		}
		action.Attributes["chatTitle"] = chat.ChatTitle
		action.Attributes["userId"] = strconv.FormatInt(chat.UserID, 10)
		action.Attributes["offTime"] = offTime
		action.Attributes["workday"] = workday
	}

	return action, nil
}

// parseSetOffFromWorkTimeArgs validates and normalises contract command args.
// For example, "1830 MON,MON,TUE" becomes offTime "1830" and workday "MON,TUE".
func parseSetOffFromWorkTimeArgs(args string) (offTime string, workday string, err error) {
	parts := strings.Split(args, " ")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%s requires off time and workday list", SetOffFromWorkTimeUTC)
	}

	offTime = parts[0]
	if !offTimePattern.MatchString(offTime) {
		return "", "", fmt.Errorf("invalid off time %q", offTime)
	}

	workday, err = normaliseWorkdayList(parts[1])
	if err != nil {
		return "", "", err
	}

	return offTime, workday, nil
}

// canonicalName restores the contract command casing.
// For example, "TOPTEN" becomes "topTen".
func canonicalName(name string) string {
	for _, definition := range commandDefinitions {
		if strings.EqualFold(definition.Name, name) {
			return definition.Name
		}
	}

	return name
}

// normaliseWorkdayList removes duplicate day tokens in order.
// For example, "MON,MON,TUE" becomes "MON,TUE".
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
// For example, "@jung2bot 1830 MON,TUE" becomes "1830 MON,TUE".
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
