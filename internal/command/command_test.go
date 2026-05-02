package command

import (
	"testing"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
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

func TestParseAllReturnsContractCommandOrder(t *testing.T) {
	t.Parallel()

	commands := ParseAll("/allJung and /topTen and /jungHelp")

	assert.Equal(t, []Command{
		{Name: JungHelp, Args: ""},
		{Name: TopTen, Args: "and /jungHelp"},
		{Name: AllJung, Args: "and /topTen and /jungHelp"},
	}, commands)
}

func TestParseIgnoresUnknownCommands(t *testing.T) {
	_, ok := Parse("/unknown")
	assert.False(t, ok)
}

func TestActionForMapsStableNames(t *testing.T) {
	tests := []struct {
		commandName string
		actionName  string
		body        string
	}{
		{commandName: JungHelp, actionName: queue.ActionJungHelp, body: queue.BodyJungHelp},
		{commandName: TopTen, actionName: queue.ActionTopTen, body: queue.BodyTopTen},
		{commandName: TopDiver, actionName: queue.ActionTopDiver, body: queue.BodyTopDiver},
		{commandName: AllJung, actionName: queue.ActionAllJung, body: queue.BodyAllJung},
		{commandName: EnableAllJung, actionName: queue.ActionEnableAllJung, body: queue.BodyEnableAllJung},
		{commandName: DisableAllJung, actionName: queue.ActionDisableAllJung, body: queue.BodyDisableAllJung},
		{commandName: SetOffFromWorkTimeUTC, actionName: queue.ActionSetOffWorkTime, body: queue.BodySetOffWorkTime},
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
			command:  Command{Name: JungHelp},
			wantBody: queue.BodyJungHelp,
			attributes: map[string]string{
				"chatId":    "123",
				"chatTitle": "title",
				"action":    queue.ActionJungHelp,
			},
		},
		{
			name:     "topten",
			command:  Command{Name: TopTen},
			wantBody: queue.BodyTopTen,
			attributes: map[string]string{
				"chatId": "123",
				"action": queue.ActionTopTen,
			},
		},
		{
			name:     "enableAllJung",
			command:  Command{Name: EnableAllJung},
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
			command:  Command{Name: SetOffFromWorkTimeUTC, Args: "1830 MON,MON,TUE"},
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
			args, err := ParseSetOffFromWorkTimeArgs(test.args)
			require.NoError(t, err)
			assert.Equal(t, test.wantOffTime, args.OffTime)
			assert.Equal(t, test.wantWorkday, args.Workday)
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
			_, err := ParseSetOffFromWorkTimeArgs(args)
			require.Error(t, err)
		})
	}
}

func TestActionForRejectsUnsupportedCommand(t *testing.T) {
	_, err := ActionFor(Command{Name: "unknown"}, ChatContext{})
	require.Error(t, err)
}

func TestActionForRejectsInvalidSetOffFromWorkTimeArgs(t *testing.T) {
	_, err := ActionFor(Command{Name: SetOffFromWorkTimeUTC, Args: "9999 MON"}, ChatContext{})
	require.Error(t, err)
}

func TestCanonicalNameReturnsUnknownName(t *testing.T) {
	assert.Equal(t, "unknown", canonicalName("unknown"))
}
