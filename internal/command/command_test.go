package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

func TestParseAll(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		commands []Command
	}{
		{
			name:     "junghelp",
			text:     "/jungHelp",
			commands: []Command{{Name: jungHelp}},
		},
		{
			name:     "topten",
			text:     "/topTen",
			commands: []Command{{Name: topTen}},
		},
		{
			name:     "topdiver",
			text:     "/topDiver",
			commands: []Command{{Name: topDiver}},
		},
		{
			name:     "alljung",
			text:     "/allJung",
			commands: []Command{{Name: allJung}},
		},
		{
			name:     "enablealljung",
			text:     "/enableAllJung",
			commands: []Command{{Name: enableAllJung}},
		},
		{
			name:     "disablealljung",
			text:     "/disableAllJung",
			commands: []Command{{Name: disableAllJung}},
		},
		{
			name:     "set off work args",
			text:     "/setOffFromWorkTimeUTC 1830 MON,TUE",
			commands: []Command{{Name: setOffWorkTime, Args: "1830 MON,TUE"}},
		},
		{
			name:     "set off work without args",
			text:     "/setOffFromWorkTimeUTC",
			commands: []Command{{Name: setOffWorkTime}},
		},
		{
			name:     "command inside text",
			text:     "please /topTen now",
			commands: []Command{{Name: topTen, Args: "now"}},
		},
		{
			name:     "case insensitive",
			text:     "/TOPTEN",
			commands: []Command{{Name: topTen}},
		},
		{
			name:     "deployed prefix match",
			text:     "/topTen123",
			commands: []Command{{Name: topTen, Args: "123"}},
		},
		{
			name:     "deployed set off prefix match",
			text:     "/setOffFromWorkTimeUTC123 1830 MON",
			commands: []Command{{Name: setOffWorkTime, Args: "1830 MON"}},
		},
		{
			name:     "mention stripped",
			text:     "/setOffFromWorkTimeUTC@jung2bot 1830 MON,TUE",
			commands: []Command{{Name: setOffWorkTime, Args: "1830 MON,TUE"}},
		},
		{
			name:     "unknown command",
			text:     "/unknown",
			commands: []Command{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.commands, ParseAll(test.text))
		})
	}
}

func TestParseAllReturnsContractCommandOrder(t *testing.T) {
	t.Parallel()

	commands := ParseAll("/allJung and /topTen and /jungHelp")

	assert.Equal(t, []Command{
		{Name: jungHelp, Args: ""},
		{Name: topTen, Args: "and /jungHelp"},
		{Name: allJung, Args: "and /topTen and /jungHelp"},
	}, commands)
}

func TestCommandArgs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "plain args",
			raw:  " now",
			want: "now",
		},
		{
			name: "mention with args",
			raw:  "@jung2bot 1830 MON,TUE",
			want: "1830 MON,TUE",
		},
		{
			name: "mention only",
			raw:  "@jung2bot",
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, commandArgs(test.raw))
		})
	}
}

func TestActionForMapsStableNames(t *testing.T) {
	tests := []struct {
		commandName string
		actionName  string
		body        string
	}{
		{commandName: jungHelp, actionName: queue.ActionJungHelp, body: queue.BodyJungHelp},
		{commandName: topTen, actionName: queue.ActionTopTen, body: queue.BodyTopTen},
		{commandName: topDiver, actionName: queue.ActionTopDiver, body: queue.BodyTopDiver},
		{commandName: allJung, actionName: queue.ActionAllJung, body: queue.BodyAllJung},
		{commandName: enableAllJung, actionName: queue.ActionEnableAllJung, body: queue.BodyEnableAllJung},
		{commandName: disableAllJung, actionName: queue.ActionDisableAllJung, body: queue.BodyDisableAllJung},
		{commandName: setOffWorkTime, actionName: queue.ActionSetOffWorkTime, body: queue.BodySetOffWorkTime},
	}

	for _, test := range tests {
		t.Run(test.commandName, func(t *testing.T) {
			action, err := ActionFor(
				Command{Name: test.commandName, Args: "1830 MON,TUE"},
				ChatContext{ChatID: 123, ChatTitle: "title", UserID: 456},
			)
			require.NoError(t, err)

			assert.Equal(t, test.actionName, action.Name)
			assert.Equal(t, test.body, action.Body)
			assert.Equal(t, "123", action.Attributes["chatId"])
			assert.Equal(t, test.actionName, action.Attributes["action"])
		})
	}
}

func TestActionForPreservesContractAttributeShapes(t *testing.T) {
	tests := []struct {
		name       string
		command    Command
		wantBody   string
		attributes map[string]string
	}{
		{
			name:     "junghelp",
			command:  Command{Name: jungHelp},
			wantBody: queue.BodyJungHelp,
			attributes: map[string]string{
				"chatId":    "123",
				"chatTitle": "title",
				"action":    queue.ActionJungHelp,
			},
		},
		{
			name:     "topten",
			command:  Command{Name: topTen},
			wantBody: queue.BodyTopTen,
			attributes: map[string]string{
				"chatId": "123",
				"action": queue.ActionTopTen,
			},
		},
		{
			name:     "enableAllJung",
			command:  Command{Name: enableAllJung},
			wantBody: queue.BodyEnableAllJung,
			attributes: map[string]string{
				"chatId":    "123",
				"chatTitle": "title",
				"userId":    "456",
				"action":    queue.ActionEnableAllJung,
			},
		},
		{
			name:     "setOffFromWorkTimeUTC",
			command:  Command{Name: setOffWorkTime, Args: "1830 MON,MON,TUE"},
			wantBody: queue.BodySetOffWorkTime,
			attributes: map[string]string{
				"chatId":    "123",
				"chatTitle": "title",
				"userId":    "456",
				"offTime":   "1830",
				"workday":   "MON,TUE",
				"action":    queue.ActionSetOffWorkTime,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action, err := ActionFor(test.command, ChatContext{ChatID: 123, ChatTitle: "title", UserID: 456})
			require.NoError(t, err)

			assert.Equal(t, test.wantBody, action.Body)
			assert.Equal(t, test.attributes, action.Attributes)
		})
	}
}

func TestParseSetOffFromWorkTimeArgsMatchesContractValidation(t *testing.T) {
	tests := []struct {
		args        string
		wantOffTime string
		wantWorkday string
	}{
		{args: "0000 MON", wantOffTime: "0000", wantWorkday: "MON"},
		{args: "2345 MON,TUE,WED,THU,FRI,SAT,SUN", wantOffTime: "2345", wantWorkday: "MON,TUE,WED,THU,FRI,SAT,SUN"},
		{args: "1830 MON,MON", wantOffTime: "1830", wantWorkday: "MON"},
	}

	for _, test := range tests {
		t.Run(test.args, func(t *testing.T) {
			offTime, workday, err := parseSetOffFromWorkTimeArgs(test.args)
			require.NoError(t, err)
			assert.Equal(t, test.wantOffTime, offTime)
			assert.Equal(t, test.wantWorkday, workday)
		})
	}
}

func TestParseSetOffFromWorkTimeArgsRejectsContractInvalidFormats(t *testing.T) {
	tests := []string{
		"",
		"0000",
		"2400 MON",
		"1748 MON",
		"1830 mon",
		"1830 MON,",
		"1830 MON FRI",
		"1830  MON",
		"1830\tMON",
		"1830 MON ",
		"\t1830 MON",
		"1830 MON,TUE,WED,THU,FRI,SAT,SUN,MON",
	}

	for _, args := range tests {
		t.Run(args, func(t *testing.T) {
			_, _, err := parseSetOffFromWorkTimeArgs(args)
			require.Error(t, err)
		})
	}
}

func TestActionForEnrichesAttributesForMixedCaseCommandNames(t *testing.T) {
	action, err := ActionFor(
		Command{Name: "ENABLEALLJUNG"},
		ChatContext{ChatID: 123, ChatTitle: "title", UserID: 456},
	)
	require.NoError(t, err)

	assert.Equal(t, queue.ActionEnableAllJung, action.Name)
	assert.Equal(t, map[string]string{
		"chatId":    "123",
		"chatTitle": "title",
		"userId":    "456",
		"action":    queue.ActionEnableAllJung,
	}, action.Attributes)
}

func TestActionForRejectsUnsupportedCommand(t *testing.T) {
	_, err := ActionFor(Command{Name: "unknown"}, ChatContext{})
	require.Error(t, err)
}

func TestActionForRejectsInvalidSetOffFromWorkTimeArgs(t *testing.T) {
	_, err := ActionFor(Command{Name: setOffWorkTime, Args: "9999 MON"}, ChatContext{})
	require.Error(t, err)
}

func TestCanonicalNameReturnsUnknownName(t *testing.T) {
	assert.Equal(t, "unknown", canonicalName("unknown"))
}
