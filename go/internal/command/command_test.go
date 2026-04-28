package command

import (
	"testing"

	"github.com/siutsin/telegram-jung2-bot/go/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSupportedCommands(t *testing.T) {
	tests := []struct {
		text string
		name string
		args string
	}{
		{text: "/jungHelp", name: JungHelp},
		{text: "/topTen", name: TopTen},
		{text: "/topDiver", name: TopDiver},
		{text: "/allJung", name: AllJung},
		{text: "/enableAllJung", name: EnableAllJung},
		{text: "/disableAllJung", name: DisableAllJung},
		{text: "/setOffFromWorkTimeUTC 1830 MON,TUE", name: SetOffFromWorkTimeUTC, args: "1830 MON,TUE"},
		{text: "please /topTen now", name: TopTen, args: "now"},
		{text: "/TOPTEN", name: TopTen},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			command, ok := Parse(test.text)
			require.True(t, ok)
			assert.Equal(t, test.name, command.Name)
			assert.Equal(t, test.args, command.Args)
		})
	}
}

func TestParseIgnoresUnknownCommands(t *testing.T) {
	_, ok := Parse("/unknown")
	assert.False(t, ok)
}

func TestActionForMapsStableNames(t *testing.T) {
	tests := []struct {
		commandName string
		actionName  string
	}{
		{commandName: JungHelp, actionName: queue.ActionJungHelp},
		{commandName: TopTen, actionName: queue.ActionTopTen},
		{commandName: TopDiver, actionName: queue.ActionTopDiver},
		{commandName: AllJung, actionName: queue.ActionAllJung},
		{commandName: EnableAllJung, actionName: queue.ActionEnableAllJung},
		{commandName: DisableAllJung, actionName: queue.ActionDisableAllJung},
		{commandName: SetOffFromWorkTimeUTC, actionName: queue.ActionSetOffWorkTime},
	}

	for _, test := range tests {
		t.Run(test.commandName, func(t *testing.T) {
			action, err := ActionFor(
				Command{Name: test.commandName, Args: "1830 MON,TUE"},
				ChatContext{ChatID: 123, ChatTitle: "title", UserID: 456},
			)
			require.NoError(t, err)

			assert.Equal(t, test.actionName, action.Name)
			assert.JSONEq(t, `{"chatId":123,"chatTitle":"title","userId":456,"args":"1830 MON,TUE"}`, string(action.Payload))
		})
	}
}

func TestActionForRejectsUnsupportedCommand(t *testing.T) {
	_, err := ActionFor(Command{Name: "unknown"}, ChatContext{})
	require.Error(t, err)
}
