package statistics

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

func TestNormaliseRowsRanksByMessageCount(t *testing.T) {
	t.Parallel()

	rows := sampleRows()
	normalised := normaliseRows(rows, false)

	require.Len(t, normalised.rankings, 3)
	assert.Equal(t, 5, normalised.totalMessage)
	assert.Equal(t, int64(1), normalised.rankings[0].userID)
	assert.Equal(t, 3, normalised.rankings[0].count)
	assert.Equal(t, "Ada Lovelace", normalised.rankings[0].fullName)
	assert.Equal(t, int64(2), normalised.rankings[1].userID)
	assert.Equal(t, " ", normalised.rankings[1].fullName)
	assert.Equal(t, int64(3), normalised.rankings[2].userID)
	assert.Equal(t, " ", normalised.rankings[2].fullName)
}

func TestNormaliseRowsRanksDiversByLowMessageCount(t *testing.T) {
	t.Parallel()

	normalised := normaliseRows(sampleRows(), true)

	assert.Equal(t, []int64{2, 3, 1}, []int64{
		normalised.rankings[0].userID,
		normalised.rankings[1].userID,
		normalised.rankings[2].userID,
	})
}

func TestGenerateTopTenReport(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport(sampleRows(), Options{
		Limit: 10,
		Now:   now(),
	})

	require.NoError(t, err)
	assert.Equal(t, 3, summary.UserCount)
	assert.Equal(t, 5, summary.MessageCount)
	assert.Contains(t, summary.Report, "圍爐區: Group\n\nTop 10 冗員s in the last 7 days (last 上水 time):")
	assert.Contains(t, summary.Report, "1. Ada Lovelace 60.00% (a day ago)")
	assert.Contains(t, summary.Report, "2.   20.00% (2 days ago)")
	assert.Contains(t, summary.Report, "3.   20.00% (a few seconds ago)")
	assert.Contains(t, summary.Report, "Total messages: 5")
	assert.Contains(t, summary.Report, "Last Update: 2026-05-02T12:00:00+00:00")
	assert.NotContains(t, summary.Report, "4.")
}

func TestTopTenRender(t *testing.T) {
	t.Parallel()

	report := topTen(sampleRows())
	report.options.Now = now()

	rendered := render(report)

	assert.Contains(t, rendered, "Top 10 冗員s")
	assert.Contains(t, rendered, "1. Ada Lovelace 60.00%")
}

func TestGenerateAllJungReport(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport(sampleRows(), Options{Now: now(), WindowDays: 14})

	require.NoError(t, err)
	assert.Contains(t, summary.Report, "All 冗員s in the last 14 days")
	assert.Contains(t, summary.Report, "3.   20.00%")
}

func TestAllJungRender(t *testing.T) {
	t.Parallel()

	report := allJung(sampleRows())
	report.options.Now = now()

	rendered := render(report)

	assert.Contains(t, rendered, "All 冗員s")
	assert.Contains(t, rendered, "3.   20.00%")
}

func TestGenerateTopDiverReport(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport(sampleRows(), Options{
		Limit:   2,
		Reverse: true,
		Now:     now(),
	})

	require.NoError(t, err)
	assert.Contains(t, summary.Report, "Top 2 潛水員s in the last 7 days:")
	assert.Contains(t, summary.Report, "By 冗power:\n1.   20.00%")
	assert.Contains(t, summary.Report, "By last 上水:\n1.   - 2 days ago\n2. Ada Lovelace - a day ago")
	assert.Contains(t, summary.Report, "between, 深潛會搵唔到 ho chi is")
	assert.NotContains(t, summary.Report, "3. Ada Lovelace 60.00%")
}

func TestTopDiverUsesOnlyMessageRowsForContract(t *testing.T) {
	t.Parallel()

	report := topDiver(sampleRows())

	report.options.Now = now()

	rendered := render(report)

	assert.NotContains(t, rendered, "Silent User")
}

func TestRenderReturnsEmptyStringForInvalidReport(t *testing.T) {
	t.Parallel()

	assert.Empty(t, render(reportRequest{}))
}

func TestGenerateOffFromWorkReport(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport(sampleRows(), Options{Limit: 1, OffFromWork: true, Now: now()})

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(summary.Report, "夠鐘收工~~\n\n"))
}

func TestGenerateReportRejectsEmptyRows(t *testing.T) {
	t.Parallel()

	_, err := GenerateReport(nil, Options{})

	require.ErrorIs(t, err, ErrEmptyRows)
}

func TestGenerateReportDefaultsNow(t *testing.T) {
	t.Parallel()

	summary, err := GenerateReport([]message.Message{{
		ChatTitle:   "Group",
		UserID:      1,
		FirstName:   "Ada",
		DateCreated: time.Now(),
	}}, Options{})

	require.NoError(t, err)
	assert.Contains(t, summary.Report, "Last Update:")
}

func TestGenerateReportTruncatesFinalText(t *testing.T) {
	t.Parallel()

	rows := make([]message.Message, 0, telegram.ReportLimit)
	for index := range telegram.ReportLimit {
		rows = append(rows, message.Message{
			ChatTitle:   "Group",
			UserID:      int64(index + 1),
			FirstName:   strings.Repeat("冗", 20),
			DateCreated: now().Add(-time.Hour),
		})
	}

	summary, err := GenerateReport(rows, Options{Now: now()})

	require.NoError(t, err)
	assert.True(t, utf8.ValidString(summary.Report))
	assert.LessOrEqual(t, utf8.RuneCountInString(summary.Report), telegram.ReportLimit)
}

func TestDisplayNameMatchesContractJoinBehaviour(t *testing.T) {
	t.Parallel()

	assert.Equal(t, " grace", displayName(message.Message{LastName: "grace"}))
	assert.Equal(t, " ", displayName(message.Message{Username: "grace", UserID: 3}))
}

func TestBuildBodyWithLimitCountsCharactersInsteadOfBytes(t *testing.T) {
	t.Parallel()

	normalised := normaliseRows([]message.Message{
		{ChatTitle: "Group", UserID: 1, FirstName: strings.Repeat("冗", 4), DateCreated: now().Add(-time.Hour)},
		{ChatTitle: "Group", UserID: 2, FirstName: strings.Repeat("冗", 4), DateCreated: now().Add(-2 * time.Hour)},
	}, false)

	body := buildBodyWithLimit(normalised, Options{Now: now()}, 40)

	assert.Contains(t, body, "2. 冗冗冗冗")
}

func TestJsStringLengthCountsAstralCharactersAsTwoUnits(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 2, jsStringLength("😀"))
	assert.Equal(t, 3, jsStringLength("a😀"))
}

func TestTruncateReportByJSLengthKeepsValidUTF8(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("a", telegram.ReportLimit-1) + "😀"
	truncated := truncateReportByJSLength(text)

	assert.True(t, utf8.ValidString(truncated))
	assert.LessOrEqual(t, jsStringLength(truncated), telegram.ReportLimit)
	assert.NotEqual(t, text, truncated)
}

func TestBuildBodyWithLimitClampsNegativeLimit(t *testing.T) {
	t.Parallel()

	body := buildBodyWithLimit(normaliseRows(sampleRows(), false), Options{Now: now()}, -1)

	assert.Equal(t, "...\n...\n", body)
}

func TestBuildDiverBodyLimitsToAvailableRows(t *testing.T) {
	t.Parallel()

	body := buildDiverBody(normaliseRows(sampleRows()[:1], true), Options{Limit: 10, Now: now()})

	assert.Equal(t, "1. Ada Lovelace - a day ago\n", body)
}

func TestHelpMessage(t *testing.T) {
	t.Parallel()

	helpMessage := HelpMessage("Group")

	assert.Equal(t, "\n"+
		"圍爐區: Group\n\n"+
		"冗員[jung2jyun4] Excess personnel in Cantonese\n\n"+
		"This bot is created for counting the number of message per participant in the group.\n\n"+
		"Commands:\n"+
		"/topTen  show top ten 冗員s\n"+
		"/topDiver  show top ten 潛水員s (潛得太深會搵唔到)\n"+
		"/allJung  show all 冗員s\n"+
		"/jungHelp  show help message\n\n"+
		"Admin Only:\n"+
		"/enableAllJung  enable `/alljung` command\n"+
		"/disableAllJung  disable `/alljung` command\n"+
		"/setOffFromWorkTimeUTC  set offFromWork time (UTC time)\n\n"+
		"[Bug Report/Suggestion](https://github.com/siutsin/telegram-jung2-bot/issues)\n"+
		"[Service Status](https://stats.uptimerobot.com/kglZJSkYZg)\n\n"+
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

func sampleRows() []message.Message {
	return []message.Message{
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
			DateCreated: now(),
		},
		{
			ChatTitle:   "Group",
			UserID:      1,
			FirstName:   "Ada",
			LastName:    "Lovelace",
			DateCreated: now().Add(-time.Hour),
		},
	}
}

func now() time.Time {
	return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
}
