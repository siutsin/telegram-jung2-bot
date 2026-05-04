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

type Report struct {
	Rows    []message.Message
	Options Options
}

type NormalisedRows struct {
	TotalMessage int
	Rankings     []Ranking
}

type Ranking struct {
	UserID      int64
	ChatTitle   string
	FirstName   string
	LastName    string
	Username    string
	FullName    string
	DateCreated time.Time
	Count       int
}

// NormaliseRows groups rows by user and counts messages.
// For example, two rows from the same user become one ranking with Count 2.
func NormaliseRows(rows []message.Message, reverse bool) NormalisedRows {
	tally := make(map[int64]int)
	firstSeen := make([]Ranking, 0, len(rows))
	seen := make(map[int64]bool)

	for _, row := range rows {
		tally[row.UserID]++
		if seen[row.UserID] {
			continue
		}
		seen[row.UserID] = true
		firstSeen = append(firstSeen, Ranking{
			UserID:      row.UserID,
			ChatTitle:   row.ChatTitle,
			FirstName:   row.FirstName,
			LastName:    row.LastName,
			Username:    row.Username,
			FullName:    displayName(row),
			DateCreated: row.DateCreated,
		})
	}

	for index := range firstSeen {
		firstSeen[index].Count = tally[firstSeen[index].UserID]
	}
	sort.SliceStable(firstSeen, func(left int, right int) bool {
		if reverse {
			return firstSeen[left].Count < firstSeen[right].Count
		}
		return firstSeen[left].Count > firstSeen[right].Count
	})

	return NormalisedRows{
		TotalMessage: len(rows),
		Rankings:     firstSeen,
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

	normalisedRows := NormaliseRows(rows, options.Reverse)
	report := BuildHeader(normalisedRows, options)
	footer := BuildFooter(normalisedRows, options)
	report += BuildBodyWithLimit(normalisedRows, options, telegram.ReportLimit)
	report += footer
	if options.OffFromWork {
		report = "夠鐘收工~~\n\n" + report
	}
	report = telegram.TruncateReport(report)

	return Summary{
		Report:       report,
		UserCount:    len(normalisedRows.Rankings),
		MessageCount: normalisedRows.TotalMessage,
	}, nil
}

// TopTen builds a top-ten report request.
// For example, 20 messages become Report{Options: Options{Limit: 10}}.
func TopTen(messages []message.Message) Report {
	return Report{
		Rows:    messages,
		Options: Options{Limit: 10},
	}
}

// TopDiver builds a top-diver report request.
// For example, messages become a reverse-ranked report with Limit 10.
func TopDiver(messages []message.Message, participants []telegram.User) Report {
	return Report{
		Rows:    messages,
		Options: Options{Limit: 10, Reverse: true},
	}
}

// AllJung builds an all-users report request.
// For example, messages become a report with the zero-value Options.
func AllJung(messages []message.Message) Report {
	return Report{Rows: messages}
}

// Render renders a report to text.
// For example, a TopTen report becomes the final Telegram message text.
func Render(report Report) string {
	summary, err := GenerateReport(report.Rows, report.Options)
	if err != nil {
		return ""
	}

	return summary.Report
}

// BuildHeader builds the report header text.
// For example, limit 10 becomes a header starting with "Top 10".
func BuildHeader(normalisedRows NormalisedRows, options Options) string {
	chatTitle := normalisedRows.Rankings[0].ChatTitle
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

// BuildBody builds the report body text.
// For example, a normal ranking becomes numbered lines like "1. Name 50.00%".
func BuildBody(normalisedRows NormalisedRows, options Options) string {
	return BuildBodyWithLimit(normalisedRows, options, telegram.ReportLimit)
}

// BuildBodyWithLimit builds the report body within limit.
// For example, a small limit truncates the body to "...\n...\n" once it would
// exceed the rune budget.
func BuildBodyWithLimit(normalisedRows NormalisedRows, options Options, limit int) string {
	if limit < 0 {
		limit = 0
	}

	body := ""
	if options.Reverse {
		body += "By 冗power:\n"
	}
	truncated := false

	loopLimit := len(normalisedRows.Rankings)
	if options.Limit > 0 && options.Limit < loopLimit {
		loopLimit = options.Limit
	}
	for index := range loopLimit {
		ranking := normalisedRows.Rankings[index]
		percentage := float64(ranking.Count) / float64(normalisedRows.TotalMessage) * 100
		item := fmt.Sprintf("%d. %s %.2f%% (%s)\n", index+1, ranking.FullName, percentage, timeAgo(ranking.DateCreated, options.Now))
		if utf8.RuneCountInString(body) >= limit {
			truncated = true
			break
		}
		body += item
	}
	if truncated {
		return body + "...\n...\n"
	}

	if options.Reverse {
		body += "\nBy last 上水:\n"
		body += BuildDiverBody(normalisedRows, options)
	}

	return body
}

// BuildDiverBody builds the reverse-ranking detail section.
// For example, reverse rankings become lines ordered by oldest DateCreated
// first.
func BuildDiverBody(normalisedRows NormalisedRows, options Options) string {
	rankings := append([]Ranking(nil), normalisedRows.Rankings...)
	sort.SliceStable(rankings, func(left int, right int) bool {
		return rankings[left].DateCreated.Before(rankings[right].DateCreated)
	})

	loopLimit := len(rankings)
	if options.Limit > 0 && options.Limit < loopLimit {
		loopLimit = options.Limit
	}

	var body strings.Builder
	for index := range loopLimit {
		ranking := rankings[index]
		body.WriteString(strconv.Itoa(index + 1))
		body.WriteString(". ")
		body.WriteString(ranking.FullName)
		body.WriteString(" - ")
		body.WriteString(timeAgo(ranking.DateCreated, options.Now))
		body.WriteByte('\n')
	}

	return body.String()
}

// BuildFooter builds the report footer text.
// For example, TotalMessage 20 becomes a footer starting with
// "Total messages: 20".
func BuildFooter(normalisedRows NormalisedRows, options Options) string {
	footer := fmt.Sprintf("\nTotal messages: %d\n\n", normalisedRows.TotalMessage)
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
// For example, FirstName "Ada" and LastName "Lovelace" become "Ada Lovelace".
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
