package chat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

func TestFromTelegramBuildsChatMetadata(t *testing.T) {
	now := mustParseTime(t, "2019-04-01T02:38:24Z")

	settings := FromTelegram(telegram.Message{Chat: telegram.Chat{ID: 123, Title: "title"}}, now)

	assert.Equal(t, ChatSetting{
		ChatID:        123,
		ChatTitle:     "title",
		DateCreated:   now,
		TTL:           1554691104,
		EnableAllJung: true,
	}, settings)
}

func TestParseRowAppliesContractDefaults(t *testing.T) {
	settings, err := ParseRow(Row{ChatID: 123})
	require.NoError(t, err)

	assert.Equal(t, ChatSetting{
		ChatID:        123,
		EnableAllJung: true,
	}, settings)
}

func TestParseRowUsesStoredValues(t *testing.T) {
	enabled := false
	mask := workday.Mon | workday.Fri

	settings, err := ParseRow(Row{
		ChatID:        123,
		ChatTitle:     "title",
		DateCreated:   "2019-03-16T02:26:19+08:00",
		TTL:           1558349640,
		EnableAllJung: &enabled,
		OffTime:       "1800",
		Workday:       &mask,
	})
	require.NoError(t, err)

	assert.Equal(t, ChatSetting{
		ChatID:        123,
		ChatTitle:     "title",
		DateCreated:   mustParseTime(t, "2019-03-16T02:26:19+08:00"),
		TTL:           1558349640,
		EnableAllJung: false,
		OffTime:       "1800",
		Workday:       workday.Workdays(mask),
		HasOffTime:    true,
		HasWorkday:    true,
	}, settings)
}

func TestParseRowRejectsMalformedValues(t *testing.T) {
	badMask := 128

	tests := []Row{
		{DateCreated: "bad-date"},
		{Workday: &badMask},
	}

	for _, test := range tests {
		_, err := ParseRow(test)
		require.Error(t, err)
	}
}

func TestFromScheduleRowPreservesScheduleParity(t *testing.T) {
	disabled := false
	mask := workday.Mon | 128

	settings := []ChatSetting{
		FromScheduleRow(Row{ChatID: 1}),
		FromScheduleRow(Row{ChatID: 2, EnableAllJung: &disabled}),
		FromScheduleRow(Row{ChatID: 3, DateCreated: "bad"}),
		FromScheduleRow(Row{ChatID: 4, Workday: &mask}),
	}

	assert.Equal(t, []ChatSetting{
		{ChatID: 1, EnableAllJung: true},
		{ChatID: 2, EnableAllJung: false},
		{ChatID: 3, EnableAllJung: true},
		{ChatID: 4, EnableAllJung: true, Workday: workday.Workdays(workday.Mon), HasWorkday: true},
	}, settings)
}

func TestBuildUpdatePreservesContractShape(t *testing.T) {
	tests := []struct {
		name   string
		update UpdateExpression
		want   UpdateExpression
	}{
		{
			name: "metadata",
			update: BuildMetadataUpdate("chat-id-dev", ChatSetting{
				ChatID:      123,
				ChatTitle:   "title",
				DateCreated: mustParseTime(t, "2019-04-01T02:38:24Z"),
				TTL:         1554691104,
			}),
			want: UpdateExpression{
				TableName:        "chat-id-dev",
				Key:              map[string]any{"chatId": int64(123)},
				UpdateExpression: "SET #ct = :ct, #dc = :dc, #ttl = :ttl",
				ExpressionAttributeNames: map[string]string{
					"#ct":  "chatTitle",
					"#dc":  "dateCreated",
					"#ttl": "ttl",
				},
				ExpressionAttributeValues: map[string]any{
					":ct":  "title",
					":dc":  "2019-04-01T10:38:24+08:00",
					":ttl": int64(1554691104),
				},
			},
		},
		{
			name:   "all jung",
			update: BuildAllJungUpdate("chat-id-dev", 123, false),
			want: UpdateExpression{
				TableName:        "chat-id-dev",
				Key:              map[string]any{"chatId": int64(123)},
				UpdateExpression: "SET #eaj = :eaj",
				ExpressionAttributeNames: map[string]string{
					"#eaj": "enableAllJung",
				},
				ExpressionAttributeValues: map[string]any{
					":eaj": false,
				},
			},
		},
		{
			name:   "off work",
			update: BuildOffWorkUpdate("chat-id-dev", 123, "1800", workday.Workdays(workday.Mon|workday.Fri)),
			want: UpdateExpression{
				TableName:        "chat-id-dev",
				Key:              map[string]any{"chatId": int64(123)},
				UpdateExpression: "SET #ot = :ot, #wd = :wd",
				ExpressionAttributeNames: map[string]string{
					"#ot": "offTime",
					"#wd": "workday",
				},
				ExpressionAttributeValues: map[string]any{
					":ot": "1800",
					":wd": workday.Mon | workday.Fri,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.update)
		})
	}
}

func TestFilterDuePreservesContractDefaults(t *testing.T) {
	rows := []ChatSetting{
		{ChatID: 1},
		{ChatID: 2, OffTime: "1000", HasOffTime: true, Workday: workday.Workdays(workday.Mon), HasWorkday: true},
		{ChatID: 3, OffTime: "1000", HasOffTime: true, Workday: workday.Workdays(workday.Tue), HasWorkday: true},
		{ChatID: 4, OffTime: "1800", HasOffTime: true, Workday: workday.Workdays(workday.Mon), HasWorkday: true},
		{ChatID: 5, OffTime: "1000", HasOffTime: true},
	}

	due := FilterDue(rows, "1000", "MON")

	assert.Equal(t, []ChatSetting{rows[0], rows[1]}, due)
}

func TestFilterDueRejectsMissingSettingsOutsideContractDefault(t *testing.T) {
	tests := []struct {
		name    string
		offTime string
		day     string
	}{
		{name: "default off time on non-weekday", offTime: "1000", day: "SAT"},
		{name: "non-default off time", offTime: "1800", day: "MON"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			due := FilterDue([]ChatSetting{{ChatID: 1}}, test.offTime, test.day)

			assert.Empty(t, due)
		})
	}
}

func mustParseTime(t *testing.T, raw string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err)

	return parsed
}
