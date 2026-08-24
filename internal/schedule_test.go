package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
)

func TestWindowFromTime(t *testing.T) {
	t.Parallel()

	view := WindowFromTime(time.Date(2022, 3, 4, 10, 0, 0, 0, time.UTC))

	assert.Equal(t, window{OffTime: "1000", Weekday: "FRI"}, view)
}

func TestWindowFromTimeNormalisesToUTC(t *testing.T) {
	t.Parallel()

	view := WindowFromTime(time.Date(2022, 3, 4, 18, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)))

	assert.Equal(t, window{OffTime: "1000", Weekday: "FRI"}, view)
}

func TestDueChatIDs(t *testing.T) {
	t.Parallel()

	rows := []ChatSetting{
		{ChatID: 1},
		{ChatID: 2, OffTime: "1000", HasOffTime: true, Workday: Workdays(Fri), HasWorkday: true},
		{ChatID: 3, OffTime: "1000", HasOffTime: true, Workday: Workdays(Mon), HasWorkday: true},
		{ChatID: 4, OffTime: "1800", HasOffTime: true, Workday: Workdays(Fri), HasWorkday: true},
	}

	chatIDs := DueChatIDs(rows, time.Date(2022, 3, 4, 10, 0, 0, 0, time.UTC))

	assert.Equal(t, []int64{1, 2}, chatIDs)
}

// Keep both action builders together because they pin the exact queue contract
// shape the runtime depends on.
func TestBuildActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		build     func() queue.Action
		wantName  string
		wantBody  string
		wantAttrs map[string]string
	}{
		{
			name:     "on off from work",
			build:    func() queue.Action { return BuildOnOffFromWorkAction("2022-03-04T10:00:00.000Z") },
			wantName: queue.ActionOnOffFromWork,
			wantBody: queue.BodyOnOffFromWork,
			wantAttrs: map[string]string{
				"action":     "onOffFromWork",
				"timeString": "2022-03-04T10:00:00.000Z",
			},
		},
		{
			name:     "off from work",
			build:    func() queue.Action { return BuildOffFromWorkAction(123) },
			wantName: queue.ActionOffFromWork,
			wantBody: queue.BodyOffFromWork,
			wantAttrs: map[string]string{
				"action": "offFromWork",
				"chatId": "123",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := test.build()

			assert.Equal(t, test.wantName, action.Name)
			assert.Equal(t, test.wantBody, action.Body)
			assert.Equal(t, test.wantAttrs, action.Attributes)
		})
	}
}

// Admin-only toggles share the same contract shape except for reply text and
// boolean payload, so one table keeps the assertions aligned.
func TestAllJungSettingChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		build     func(isAdmin bool) SettingChange
		wantReply string
		wantAttrs map[string]any
	}{
		{
			name:      "enable",
			build:     func(isAdmin bool) SettingChange { return EnableAllJung("chats", 123, "Group", isAdmin) },
			wantReply: "Enabled AllJung command",
			wantAttrs: map[string]any{":eaj": true},
		},
		{
			name:      "disable",
			build:     func(isAdmin bool) SettingChange { return DisableAllJung("chats", 123, "Group", isAdmin) },
			wantReply: "Disabled AllJung command",
			wantAttrs: map[string]any{":eaj": false},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			change := test.build(true)

			require.True(t, change.Allowed)
			assert.Contains(t, change.Reply, test.wantReply)
			assert.Equal(t, "chats", change.Update.TableName)
			assert.Equal(t, map[string]any{"chatId": int64(123)}, change.Update.Key)
			assert.Equal(t, test.wantAttrs, change.Update.ExpressionAttributeValues)
			assert.False(t, test.build(false).Allowed)
		})
	}
}

// This test protects the contract that default rows implicitly fire only on
// weekdays, which is easy to break during schedule refactors.
func TestDueChatIDsSkipsContractDefaultOnWeekend(t *testing.T) {
	t.Parallel()

	chatIDs := DueChatIDs([]ChatSetting{{ChatID: 1}}, time.Date(2022, 3, 5, 10, 0, 0, 0, time.UTC))

	assert.Empty(t, chatIDs)
}

// This keeps the admin update payload stable because callers depend on the
// exact workday bitmask and update-expression shape.
func TestSetOffFromWorkTimeUTC(t *testing.T) {
	t.Parallel()

	change, err := SetOffFromWorkTimeUTC("chats", 123, "Group", true, "1800", "MON,TUE")

	require.NoError(t, err)
	require.True(t, change.Allowed)
	assert.Contains(t, change.Reply, "Updated setOffFromWorkTime in UTC: 1800 MON,TUE")
	assert.Equal(t, "SET #ot = :ot, #wd = :wd", change.Update.UpdateExpression)
	assert.Equal(t, map[string]any{
		":ot": "1800",
		":wd": int(Mon | Tue),
	}, change.Update.ExpressionAttributeValues)
}

func TestSetOffFromWorkTimeUTCRequiresAdmin(t *testing.T) {
	t.Parallel()

	change, err := SetOffFromWorkTimeUTC("chats", 123, "Group", false, "1800", "MON")

	require.NoError(t, err)
	assert.False(t, change.Allowed)
}

func TestSetOffFromWorkTimeUTCRejectsBadWorkday(t *testing.T) {
	t.Parallel()

	_, err := SetOffFromWorkTimeUTC("chats", 123, "Group", true, "1800", "NOPE")

	require.Error(t, err)
}

func TestSetOffFromWorkTimeUTCRejectsBadOffTime(t *testing.T) {
	t.Parallel()

	_, err := SetOffFromWorkTimeUTC("chats", 123, "Group", true, "9999", "MON")

	require.Error(t, err)
}

func TestParseScheduledTimeAcceptsRFC3339Variants(t *testing.T) {
	t.Parallel()

	_, err := ParseScheduledTime("2026-05-01T18:00:00Z")
	require.NoError(t, err)

	_, err = ParseScheduledTime("2026-05-01T18:00:00.000Z")
	require.NoError(t, err)

	_, err = ParseScheduledTime("")
	require.Error(t, err)
	assert.EqualError(t, err, "timeString is required")
}

func TestInvalidSetOffFromWorkTimeUTCMessage(t *testing.T) {
	t.Parallel()

	message := InvalidSetOffFromWorkTimeUTCMessage("Group")

	assert.Contains(t, message, "圍爐區: Group")
	assert.Contains(t, message, "Error: Invalid format for setOffFromWorkTimeUTC")
	assert.Contains(t, message, "/setOffFromWorkTimeUTC 1800 MON,TUE,WED,THU,FRI")
}

func TestWeekdayToken(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "SUN", weekdayToken(time.Sunday))
	assert.Equal(t, "MON", weekdayToken(time.Monday))
	assert.Equal(t, "TUE", weekdayToken(time.Tuesday))
	assert.Equal(t, "WED", weekdayToken(time.Wednesday))
	assert.Equal(t, "THU", weekdayToken(time.Thursday))
	assert.Equal(t, "FRI", weekdayToken(time.Friday))
	assert.Equal(t, "SAT", weekdayToken(time.Saturday))
	assert.Empty(t, weekdayToken(time.Weekday(99)))
}
