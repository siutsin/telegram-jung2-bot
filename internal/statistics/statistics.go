// Package statistics renders chat activity reports.
package statistics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

const defaultWindowDays = 7

const updateTimestampLayout = "2006-01-02T15:04:05-07:00"

type Options struct {
	Limit       int
	Reverse     bool
	OffFromWork bool
	Now         time.Time
	WindowDays  int
}

type Summary struct {
	Report       string
	UserCount    int
	MessageCount int
}

type reportRequest struct {
	rows    []message.Message
	options Options
}

type rowSummary struct {
	totalMessage int
	rankings     []userRanking
}

type userRanking struct {
	userID      int64
	chatTitle   string
	firstName   string
	lastName    string
	username    string
	fullName    string
	dateCreated time.Time
	count       int
}

// normaliseRows groups rows by user and counts messages.
// For example, two rows from the same user become one ranking with count 2.
func normaliseRows(rows []message.Message, reverse bool) rowSummary {
	tally := make(map[int64]int)
	firstSeen := make([]userRanking, 0, len(rows))
	seen := make(map[int64]bool)

	for _, row := range rows {
		tally[row.UserID]++
		if seen[row.UserID] {
			continue
		}
		seen[row.UserID] = true
		firstSeen = append(firstSeen, userRanking{
			userID:      row.UserID,
			chatTitle:   row.ChatTitle,
			firstName:   row.FirstName,
			lastName:    row.LastName,
			username:    row.Username,
			fullName:    displayName(row),
			dateCreated: row.DateCreated,
		})
	}

	for index := range firstSeen {
		firstSeen[index].count = tally[firstSeen[index].userID]
	}
	sort.SliceStable(firstSeen, func(left int, right int) bool {
		if reverse {
			return firstSeen[left].count < firstSeen[right].count
		}
		return firstSeen[left].count > firstSeen[right].count
	})

	return rowSummary{
		totalMessage: len(rows),
		rankings:     firstSeen,
	}
}

// GenerateReport builds a statistics report summary.
// For example, a top-ten option set becomes Summary{Report, UserCount,
// MessageCount}.
func GenerateReport(rows []message.Message, options Options) (Summary, error) {
	if len(rows) == 0 {
		return Summary{}, fmt.Errorf("statistics rows are empty")
	}
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	if options.WindowDays == 0 {
		options.WindowDays = defaultWindowDays
	}

	normalisedRows := normaliseRows(rows, options.Reverse)
	report := buildHeader(normalisedRows, options)
	footer := buildFooter(normalisedRows, options)
	report += buildBodyWithLimit(normalisedRows, options, telegram.ReportLimit)
	report += footer
	if options.OffFromWork {
		report = "夠鐘收工~~\n\n" + report
	}
	report = telegram.TruncateReport(report)

	return Summary{
		Report:       report,
		UserCount:    len(normalisedRows.rankings),
		MessageCount: normalisedRows.totalMessage,
	}, nil
}

// topTen builds a top-ten report request.
// For example, 20 messages become Report{options: Options{Limit: 10}}.
func topTen(messages []message.Message) reportRequest {
	return reportRequest{
		rows:    messages,
		options: Options{Limit: 10},
	}
}

// topDiver builds a top-diver report request.
// For example, messages become a reverse-ranked report with Limit 10.
func topDiver(messages []message.Message) reportRequest {
	return reportRequest{
		rows:    messages,
		options: Options{Limit: 10, Reverse: true},
	}
}

// allJung builds an all-users report request.
// For example, messages become a report with the zero-value options.
func allJung(messages []message.Message) reportRequest {
	return reportRequest{rows: messages}
}

// render renders a report to text.
// For example, a TopTen report becomes the final Telegram message text.
func render(request reportRequest) string {
	summary, err := GenerateReport(request.rows, request.options)
	if err != nil {
		return ""
	}

	return summary.Report
}

// buildHeader builds the report header text.
// For example, limit 10 becomes a header starting with "Top 10".
func buildHeader(summary rowSummary, options Options) string {
	chatTitle := summary.rankings[0].chatTitle
	limitText := "All"
	if options.Limit > 0 {
		limitText = fmt.Sprintf("Top %d", options.Limit)
	}

	personType := "冗員s"
	suffix := " (last 上水 time):"
	if options.Reverse {
		personType = "潛水員s"
		suffix = ":"
	}

	return fmt.Sprintf("圍爐區: %s\n\n%s %s in the last %d days%s\n\n", chatTitle, limitText, personType, options.WindowDays, suffix)
}

// buildBodyWithLimit builds the report body within limit.
// For example, a small limit truncates the body to "...\n...\n" once it would
// exceed the rune budget.
func buildBodyWithLimit(summary rowSummary, options Options, limit int) string {
	if limit < 0 {
		limit = 0
	}

	body := ""
	if options.Reverse {
		body += "By 冗power:\n"
	}
	truncated := false

	loopLimit := len(summary.rankings)
	if options.Limit > 0 && options.Limit < loopLimit {
		loopLimit = options.Limit
	}
	for index := range loopLimit {
		item := summary.rankings[index]
		percentage := float64(item.count) / float64(summary.totalMessage) * 100
		line := fmt.Sprintf("%d. %s %.2f%% (%s)\n", index+1, item.fullName, percentage, timeAgo(item.dateCreated, options.Now))
		if utf8.RuneCountInString(body) >= limit {
			truncated = true
			break
		}
		body += line
	}
	if truncated {
		return body + "...\n...\n"
	}

	if options.Reverse {
		body += "\nBy last 上水:\n"
		body += buildDiverBody(summary, options)
	}

	return body
}

// buildDiverBody builds the reverse-ranking detail section.
// For example, reverse rankings become lines ordered by oldest dateCreated
// first.
func buildDiverBody(summary rowSummary, options Options) string {
	rankings := append([]userRanking(nil), summary.rankings...)
	sort.SliceStable(rankings, func(left int, right int) bool {
		return rankings[left].dateCreated.Before(rankings[right].dateCreated)
	})

	loopLimit := len(rankings)
	if options.Limit > 0 && options.Limit < loopLimit {
		loopLimit = options.Limit
	}

	var body strings.Builder
	for index := range loopLimit {
		item := rankings[index]
		body.WriteString(strconv.Itoa(index + 1))
		body.WriteString(". ")
		body.WriteString(item.fullName)
		body.WriteString(" - ")
		body.WriteString(timeAgo(item.dateCreated, options.Now))
		body.WriteByte('\n')
	}

	return body.String()
}

// buildFooter builds the report footer text.
// For example, totalMessage 20 becomes a footer starting with
// "Total messages: 20".
func buildFooter(summary rowSummary, options Options) string {
	footer := fmt.Sprintf("\nTotal messages: %d\n\n", summary.totalMessage)
	if options.Reverse {
		footer += "between, 深潛會搵唔到 ho chi is\n"
	}
	footer += fmt.Sprintf("Last Update: %s", options.Now.Format(updateTimestampLayout))
	return footer
}

// HelpMessage returns the bot help message.
// For example, chat title "Ops" is inserted into the contract help template.
func HelpMessage(chatTitle string) string {
	return fmt.Sprintf(`
圍爐區: %s

冗員[jung2jyun4] Excess personnel in Cantonese

This bot is created for counting the number of message per participant in the group.

Commands:
/topTen  show top ten 冗員s
/topDiver  show top ten 潛水員s (潛得太深會搵唔到)
/allJung  show all 冗員s
/jungHelp  show help message

Admin Only:
/enableAllJung  enable `+"`/alljung`"+` command
/disableAllJung  disable `+"`/alljung`"+` command
/setOffFromWorkTimeUTC  set offFromWork time (UTC time)

[Bug Report/Suggestion](https://github.com/siutsin/telegram-jung2-bot/issues)
[Service Status](https://stats.uptimerobot.com/kglZJSkYZg)

May your 冗 power powerful
`, chatTitle)
}

// displayName returns the preferred ranking display name.
// For example, firstName "Ada" and lastName "Lovelace" become "Ada Lovelace".
func displayName(row message.Message) string {
	return strings.Join([]string{row.FirstName, row.LastName}, " ")
}

// timeAgo formats a relative timestamp.
// For example, a timestamp two hours ago becomes "2 hours ago".
func timeAgo(dateCreated time.Time, referenceTime time.Time) string {
	duration := referenceTime.Sub(dateCreated)
	future := duration < 0
	if future {
		duration = -duration
	}

	text := relativeTimeText(duration)
	if future {
		return "in " + strings.TrimSuffix(text, " ago")
	}

	return text
}

func relativeTimeText(duration time.Duration) string {
	seconds := rounded(duration.Seconds())
	minutes := rounded(duration.Minutes())
	hours := rounded(duration.Hours())
	days := rounded(duration.Hours() / 24)

	switch {
	case seconds < 45:
		return "a few seconds ago"
	case minutes < 2:
		return "a minute ago"
	case minutes < 45:
		return plural(minutes, "minute")
	case hours < 2:
		return "an hour ago"
	case hours < 22:
		return plural(hours, "hour")
	case days < 2:
		return "a day ago"
	case days < 26:
		return plural(days, "day")
	case days <= 46:
		return "a month ago"
	case days < 60:
		return "2 months ago"
	case days < 320:
		return plural(days/30, "month")
	case days < 546:
		return "a year ago"
	default:
		return plural(max(rounded(duration.Hours()/(24*365)), 2), "year")
	}
}

func rounded(value float64) int {
	return int(value + 0.5)
}

// plural formats a pluralised relative time string.
// For example, plural(2, "day") becomes "2 days ago".
func plural(value int, unit string) string {
	if value == 1 {
		if unit == "hour" {
			return "an hour ago"
		}
		return "a " + unit + " ago"
	}

	return fmt.Sprintf("%d %ss ago", value, unit)
}
