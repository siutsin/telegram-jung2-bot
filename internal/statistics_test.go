package bot

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormaliseRowsRanksByMessageCount(t *testing.T) {
	t.Parallel()

	rows := sampleRows()
	normalised := normaliseRows(rows, false)

	require.Len(t, normalised.rankings, 4)
	assert.Equal(t, 6, normalised.totalMessage)
	assert.Equal(t, int64(1), normalised.rankings[0].userID)
	assert.Equal(t, 3, normalised.rankings[0].count)
	assert.Equal(t, "Ada Lovelace", normalised.rankings[0].fullName)
	assert.Equal(t, int64(2), normalised.rankings[1].userID)
	assert.Equal(t, "Grace Hopper", normalised.rankings[1].fullName)
	assert.Equal(t, int64(3), normalised.rankings[2].userID)
	assert.Equal(t, "Alan", normalised.rankings[2].fullName)
	assert.Equal(t, int64(4), normalised.rankings[3].userID)
	assert.Equal(t, "linus_t", normalised.rankings[3].fullName)
}

func TestNormaliseRowsRanksDiversByLowMessageCount(t *testing.T) {
	t.Parallel()

	normalised := normaliseRows(sampleRows(), true)

	assert.Equal(t, []int64{2, 3, 4, 1}, []int64{
		normalised.rankings[0].userID,
		normalised.rankings[1].userID,
		normalised.rankings[2].userID,
		normalised.rankings[3].userID,
	})
}

func TestGenerateTopTenReport(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport(sampleRows(), ReportOptions{
		Limit: 10,
		Now:   now(),
	})

	require.NoError(t, err)
	assert.Equal(t, 4, summary.UserCount)
	assert.Equal(t, 6, summary.MessageCount)
	assert.Contains(t, summary.Report, "圍爐區: Group\n\nTop 10 冗員s in the last 7 days (last 上水 time):")
	assert.Contains(t, summary.Report, "1. Ada Lovelace 50.00% (a day ago)")
	assert.Contains(t, summary.Report, "2. Grace Hopper 16.67% (2 days ago)")
	assert.Contains(t, summary.Report, "3. Alan 16.67% (a few seconds ago)")
	assert.Contains(t, summary.Report, "4. linus_t 16.67% (3 hours ago)")
	assert.Contains(t, summary.Report, "Total messages: 6")
	assert.Contains(t, summary.Report, "Last Update: 2026-05-02T12:00:00+00:00")
	assert.NotContains(t, summary.Report, "/setOffFromWorkTimeUTC")
	assert.NotContains(t, summary.Report, "5.")
}

// Reports are sent without parse_mode, so the title must survive verbatim.
func TestGenerateReportKeepsChatTitleUnescaped(t *testing.T) {
	t.Parallel()

	rows := sampleRows()
	for index := range rows {
		rows[index].ChatTitle = "Dev_Team *Ops*"
	}

	summary, err := GenerateReport(rows, ReportOptions{Limit: 10, Now: now()})

	require.NoError(t, err)
	assert.Contains(t, summary.Report, "圍爐區: Dev_Team *Ops*")
	assert.NotContains(t, summary.Report, `\_`)
	assert.NotContains(t, summary.Report, `\*`)
}

// HelpMessage is the only markdown message, so its title must be escaped.
func TestHelpMessageEscapesChatTitle(t *testing.T) {
	t.Parallel()

	assert.Contains(t, HelpMessage("Dev_Team *Ops*"), `圍爐區: Dev\_Team \*Ops\*`)
}

func TestGenerateAllJungReport(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport(sampleRows(), ReportOptions{Now: now(), WindowDays: 14})

	require.NoError(t, err)
	assert.Contains(t, summary.Report, "All 冗員s in the last 14 days")
	assert.Contains(t, summary.Report, "4. linus_t 16.67%")
}

func TestGenerateTopDiverReport(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport(sampleRows(), ReportOptions{
		Limit:   2,
		Reverse: true,
		Now:     now(),
	})

	require.NoError(t, err)
	assert.Contains(t, summary.Report, "Top 2 潛水員s in the last 7 days:")
	assert.Contains(t, summary.Report, "By 冗power:\n1. Grace Hopper 16.67%")
	assert.Contains(t, summary.Report, "By last 上水:\n1. Grace Hopper - 2 days ago\n2. Ada Lovelace - a day ago")
	assert.Contains(t, summary.Report, "between, 深潛會搵唔到 ho chi is")
	assert.NotContains(t, summary.Report, "3. Ada Lovelace 60.00%")
}

func TestTopDiverUsesOnlyMessageRowsForContract(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport(sampleRows(), ReportOptions{Limit: 10, Reverse: true, Now: now()})

	require.NoError(t, err)
	assert.NotContains(t, summary.Report, "Silent User")
}

func TestGenerateOffFromWorkReport(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport(sampleRows(), ReportOptions{Limit: 1, OffFromWork: true, Now: now()})

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(summary.Report, "夠鐘收工~~\n\n"))
	assert.True(t, strings.HasSuffix(summary.Report, "\n\n---\nUse /setOffFromWorkTimeUTC to set the off-work time.\nSee /jungHelp for more info."))
}

func TestGenerateReportRejectsEmptyRows(t *testing.T) {
	t.Parallel()

	_, err := GenerateReport(nil, ReportOptions{})

	require.ErrorIs(t, err, ErrEmptyRows)
}

func TestGenerateReportDefaultsNow(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport([]StoredMessage{{
		ChatTitle:   "Group",
		UserID:      1,
		FirstName:   "Ada",
		DateCreated: time.Now(),
	}}, ReportOptions{})

	require.NoError(t, err)
	assert.Contains(t, summary.Report, "Last Update:")
}

func TestGenerateReportTruncatesFinalText(t *testing.T) {
	t.Parallel()

	rows := make([]StoredMessage, 0, ReportLimit)
	for index := range ReportLimit {
		rows = append(rows, StoredMessage{
			ChatTitle:   "Group",
			UserID:      int64(index + 1),
			FirstName:   strings.Repeat("冗", 20),
			DateCreated: now().Add(-time.Hour),
		})
	}

	summary, err := GenerateReport(rows, ReportOptions{Now: now()})

	require.NoError(t, err)
	assert.True(t, utf8.ValidString(summary.Report))
	assert.LessOrEqual(t, jsStringLength(summary.Report), ReportLimit)
}

func TestGenerateReportTruncatesAstralCharactersByJSLength(t *testing.T) {
	t.Parallel()

	rows := []StoredMessage{{
		ChatTitle:   "Group",
		UserID:      1,
		FirstName:   strings.Repeat("😀", 2000),
		DateCreated: now().Add(-time.Hour),
	}}

	summary, err := GenerateReport(rows, ReportOptions{Now: now()})

	require.NoError(t, err)
	assert.LessOrEqual(t, jsStringLength(summary.Report), ReportLimit)
}

func TestDisplayNameMatchesContractJoinBehaviour(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  StoredMessage
		want string
	}{
		{
			name: "first and last",
			row:  StoredMessage{FirstName: "Ada", LastName: "Lovelace"},
			want: "Ada Lovelace",
		},
		{
			name: "first name only",
			row:  StoredMessage{FirstName: "Grace"},
			want: "Grace",
		},
		{
			name: "last name only",
			row:  StoredMessage{LastName: "Hopper"},
			want: "Hopper",
		},
		{
			name: "username only",
			row:  StoredMessage{Username: "grace", UserID: 3},
			want: "grace",
		},
		{
			name: "username with at sign",
			row:  StoredMessage{Username: "@grace"},
			want: "grace",
		},
		{
			name: "no name or username",
			row:  StoredMessage{UserID: 3},
			want: " ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, displayName(test.row))
		})
	}
}

func TestBuildBodyWithLimitCountsCharactersInsteadOfBytes(t *testing.T) {
	t.Parallel()

	normalised := normaliseRows([]StoredMessage{
		{ChatTitle: "Group", UserID: 1, FirstName: strings.Repeat("冗", 4), DateCreated: now().Add(-time.Hour)},
		{ChatTitle: "Group", UserID: 2, FirstName: strings.Repeat("冗", 4), DateCreated: now().Add(-2 * time.Hour)},
	}, false)

	body := buildBodyWithLimit(normalised, ReportOptions{Now: now()}, 40)

	assert.Contains(t, body, "2. 冗冗冗冗")
}

func TestJsStringLengthCountsAstralCharactersAsTwoUnits(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 2, jsStringLength("😀"))
	assert.Equal(t, 3, jsStringLength("a😀"))
}

func TestTruncateReportByJSLengthKeepsValidUTF8(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("a", ReportLimit-1) + "😀"
	truncated := truncateReportByJSLength(text)

	assert.True(t, utf8.ValidString(truncated))
	assert.LessOrEqual(t, jsStringLength(truncated), ReportLimit)
	assert.NotEqual(t, text, truncated)
}

func TestBuildBodyWithLimitClampsNegativeLimit(t *testing.T) {
	t.Parallel()

	body := buildBodyWithLimit(normaliseRows(sampleRows(), false), ReportOptions{Now: now()}, -1)

	assert.Equal(t, "...\n...\n", body)
}

func TestBuildDiverBodyLimitsToAvailableRows(t *testing.T) {
	t.Parallel()

	body := buildDiverBody(normaliseRows(sampleRows()[:1], true), ReportOptions{Limit: 10, Now: now()})

	assert.Equal(t, "1. Ada Lovelace - a day ago\n", body)
}

func TestHelpMessage(t *testing.T) {
	t.Parallel()

	helpMessage := HelpMessage("Group")

	assert.Equal(t, "\n"+
		"圍爐區: Group\n\n"+
		"冗員[jung2jyun4] Excess personnel in Cantonese\n\n"+
		"This bot is created for counting the number of messages per participant in the group.\n\n"+
		"Commands:\n"+
		"/topTen  show top ten 冗員s\n"+
		"/topDiver  show top ten 潛水員s (潛得太深會搵唔到)\n"+
		"/allJung  show all 冗員s\n"+
		"/jungHelp  show help message\n\n"+
		"Admin Only:\n"+
		"/enableAllJung  enable `/allJung` command\n"+
		"/disableAllJung  disable `/allJung` command\n"+
		"/setOffFromWorkTimeUTC  set UTC off-work time. E.g. 1800 MON,TUE,WED,THU,FRI\n\n"+
		"[Bug Report/Suggestion](https://github.com/siutsin/telegram-jung2-bot/issues)\n"+
		"[Service Status](https://www.webgazer.io/s?id=597)\n\n"+
		"May your 冗 power powerful\n",
		helpMessage,
	)
}

func TestTimeAgoUnits(t *testing.T) {
	t.Parallel()

	current := now()

	assert.Equal(t, "5 minutes ago", timeAgo(current.Add(-5*time.Minute), current))
	assert.Equal(t, "an hour ago", timeAgo(current.Add(-(44*time.Minute+31*time.Second)), current))
	assert.Equal(t, "an hour ago", timeAgo(current.Add(-time.Hour), current))
	assert.Equal(t, "3 hours ago", timeAgo(current.Add(-3*time.Hour), current))
	assert.Equal(t, "a day ago", timeAgo(current.Add(-(21*time.Hour+31*time.Minute)), current))
	assert.Equal(t, "a day ago", timeAgo(current.Add(-23*time.Hour), current))
	assert.Equal(t, "a month ago", timeAgo(current.Add(-(25*24*time.Hour+12*time.Hour)), current))
	assert.Equal(t, "a month ago", timeAgo(current.Add(-27*24*time.Hour), current))
	assert.Equal(t, "a month ago", timeAgo(current.Add(-31*24*time.Hour), current))
	assert.Equal(t, "a month ago", timeAgo(current.Add(-46*24*time.Hour), current))
	assert.Equal(t, "2 months ago", timeAgo(current.Add(-47*24*time.Hour), current))
	assert.Equal(t, "2 months ago", timeAgo(current.Add(-59*24*time.Hour), current))
	assert.Equal(t, "6 months ago", timeAgo(current.Add(-180*24*time.Hour), current))
	assert.Equal(t, "10 months ago", timeAgo(current.Add(-319*24*time.Hour), current))
	assert.Equal(t, "a year ago", timeAgo(current.Add(-370*24*time.Hour), current))
	assert.Equal(t, "2 years ago", timeAgo(current.Add(-546*24*time.Hour), current))
	assert.Equal(t, "2 years ago", timeAgo(current.Add(-547*24*time.Hour), current))
	assert.Equal(t, "2 years ago", timeAgo(current.Add(-800*24*time.Hour), current))
	assert.Equal(t, "in a minute", timeAgo(current.Add(time.Minute), current))
	assert.Equal(t, "in 2 months", timeAgo(current.Add(47*24*time.Hour), current))
}

func TestPluralFormatsSingularUnits(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "an hour ago", plural(1, "hour"))
	assert.Equal(t, "a day ago", plural(1, "day"))
}

func sampleRows() []StoredMessage {
	return []StoredMessage{
		{
			ChatTitle:   "Group",
			UserID:      1,
			FirstName:   "Ada",
			LastName:    "Lovelace",
			DateCreated: now().Add(-24 * time.Hour),
		},
		{
			ChatTitle:   "Group",
			UserID:      2,
			FirstName:   "Grace",
			LastName:    "Hopper",
			Username:    "grace",
			DateCreated: now().Add(-48 * time.Hour),
		},
		{
			ChatTitle:   "Group",
			UserID:      1,
			FirstName:   "Ada",
			LastName:    "Lovelace",
			DateCreated: now().Add(-12 * time.Hour),
		},
		{
			ChatTitle:   "Group",
			UserID:      3,
			FirstName:   "Alan",
			DateCreated: now(),
		},
		{
			ChatTitle:   "Group",
			UserID:      1,
			FirstName:   "Ada",
			LastName:    "Lovelace",
			DateCreated: now().Add(-time.Hour),
		},
		{
			ChatTitle:   "Group",
			UserID:      4,
			Username:    "linus_t",
			DateCreated: now().Add(-3 * time.Hour),
		},
	}
}

func now() time.Time {
	return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
}
