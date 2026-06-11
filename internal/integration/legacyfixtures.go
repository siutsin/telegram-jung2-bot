// Legacy JS queue decode fixtures for JS-to-Go cutover parity.
// Remove this file and TestFlociLegacySQSFixtures after production cutover; see
// internal/integration/README.md § Cutover cleanup.
package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/command"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
)

type legacyFixtureCase struct {
	name string
	raw  string
	want queue.Action
}

func runLegacySQSFixtureIntegration(t *testing.T) {
	t.Helper()

	for _, testCase := range legacyFixtureCases(t) {
		t.Run(testCase.name, func(t *testing.T) {
			var raw queue.RawMessage
			err := json.Unmarshal([]byte(testCase.raw), &raw)
			require.NoError(t, err, "unmarshal legacy fixture")

			got := queue.DecodeMessage(raw)
			assertAction(t, testCase.want, got)
		})
	}
}

func legacyFixtureCases(t *testing.T) []legacyFixtureCase {
	t.Helper()

	chatContext := command.ChatContext{
		ChatID:    integrationChatID,
		ChatTitle: integrationChatTitle,
		UserID:    integrationUserID,
	}

	build := func(name string, raw string, text string) legacyFixtureCase {
		action, err := command.ActionFor(command.ParseAll(text)[0], chatContext)
		require.NoError(t, err, "build legacy want action for %s", name)

		return legacyFixtureCase{name: name, raw: raw, want: action}
	}

	return []legacyFixtureCase{
		build(
			"legacy Lambda jungHelp stringValue",
			`{"receiptHandle":"legacy-junghelp","body":"sendJungHelpMessage","messageAttributes":{"action":{"stringValue":"junghelp"},"chatId":{"stringValue":"42001"},"chatTitle":{"stringValue":"Floci Integration"}}}`,
			"/jungHelp",
		),
		build(
			"legacy ECS topTen StringValue",
			`{"receiptHandle":"legacy-topten","body":"sendTopTenMessage","messageAttributes":{"action":{"StringValue":"topten"},"chatId":{"StringValue":"42001"}}}`,
			"/topTen",
		),
		build(
			"legacy Lambda topDiver stringValue",
			`{"receiptHandle":"legacy-topdiver","body":"sendTopDiverMessage","messageAttributes":{"action":{"stringValue":"topdiver"},"chatId":{"stringValue":"42001"}}}`,
			"/topDiver",
		),
		build(
			"legacy ECS allJung StringValue",
			`{"receiptHandle":"legacy-alljung","body":"sendAllJungMessage","messageAttributes":{"action":{"StringValue":"alljung"},"chatId":{"StringValue":"42001"}}}`,
			"/allJung",
		),
		build(
			"legacy Lambda enableAllJung stringValue",
			`{"receiptHandle":"legacy-enable","body":"sendEnableAllJungMessage","messageAttributes":{"action":{"stringValue":"enableAllJung"},"chatId":{"stringValue":"42001"},"chatTitle":{"stringValue":"Floci Integration"},"userId":{"stringValue":"10001"}}}`,
			"/enableAllJung",
		),
		build(
			"legacy ECS disableAllJung StringValue",
			`{"receiptHandle":"legacy-disable","body":"sendDisableAllJungMessage","messageAttributes":{"action":{"StringValue":"disableAllJung"},"chatId":{"StringValue":"42001"},"chatTitle":{"StringValue":"Floci Integration"},"userId":{"StringValue":"10001"}}}`,
			"/disableAllJung",
		),
		{
			name: "legacy Lambda setOffFromWorkTimeUTC stringValue",
			raw:  `{"receiptHandle":"legacy-setoff","body":"sendSetOffFromWorkTimeUTC","messageAttributes":{"action":{"stringValue":"setOffFromWorkTimeUTC"},"chatId":{"stringValue":"42001"},"chatTitle":{"stringValue":"Floci Integration"},"userId":{"stringValue":"10001"},"offTime":{"stringValue":"1830"},"workday":{"stringValue":"MON,TUE"}}}`,
			want: mustLegacySetOffAction(t, chatContext),
		},
		{
			name: "legacy ECS onOffFromWork StringValue",
			raw:  `{"receiptHandle":"legacy-onoff","body":"sendOnOffFromWork","messageAttributes":{"action":{"StringValue":"onOffFromWork"},"timeString":{"StringValue":"2026-06-11T18:30:00Z"}}}`,
			want: schedule.BuildOnOffFromWorkAction("2026-06-11T18:30:00Z"),
		},
		{
			name: "legacy Lambda offFromWork stringValue",
			raw:  `{"receiptHandle":"legacy-offfromwork","body":"sendOffFromWorkMessage","messageAttributes":{"action":{"stringValue":"offFromWork"},"chatId":{"stringValue":"42001"}}}`,
			want: schedule.BuildOffFromWorkAction(integrationChatID),
		},
	}
}

func mustLegacySetOffAction(t *testing.T, chatContext command.ChatContext) queue.Action {
	t.Helper()

	commands := command.ParseAll("/setOffFromWorkTimeUTC 1830 MON,TUE")
	require.Len(t, commands, 1)

	action, err := command.ActionFor(commands[0], chatContext)
	require.NoError(t, err, "build legacy setOff action")

	return action
}
