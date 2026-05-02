// Package statistics renders chat activity reports.
package statistics

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

const defaultWindowDays = 7

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
	bodyLimit := telegram.ReportLimit - utf8.RuneCountInString(report) - utf8.RuneCountInString(footer)
	if options.OffFromWork {
		bodyLimit -= utf8.RuneCountInString("夠鐘收工~~\n\n")
	}
	report += BuildBodyWithLimit(normalisedRows, options, bodyLimit)
	report += footer
	if options.OffFromWork {
		report = "夠鐘收工~~\n\n" + report
	}

	return Summary{
		Report:       telegram.TruncateReport(report),
		UserCount:    len(normalisedRows.Rankings),
		MessageCount: normalisedRows.TotalMessage,
	}, nil
}

func TopTen(messages []message.Message) Report {
	return Report{
		Rows:    messages,
		Options: Options{Limit: 10},
	}
}

func TopDiver(messages []message.Message, participants []telegram.User) Report {
	return Report{
		Rows:    messages,
		Options: Options{Limit: 10, Reverse: true},
	}
}

func AllJung(messages []message.Message) Report {
	return Report{Rows: messages}
}

func Render(report Report) string {
	summary, err := GenerateReport(report.Rows, report.Options)
	if err != nil {
		return ""
	}

	return summary.Report
}

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

func BuildBody(normalisedRows NormalisedRows, options Options) string {
	return BuildBodyWithLimit(normalisedRows, options, telegram.ReportLimit)
}

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
		if utf8.RuneCountInString(body)+utf8.RuneCountInString(item)+utf8.RuneCountInString("...\n...\n") > limit {
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

func BuildDiverBody(normalisedRows NormalisedRows, options Options) string {
	rankings := append([]Ranking(nil), normalisedRows.Rankings...)
	sort.SliceStable(rankings, func(left int, right int) bool {
		return rankings[left].DateCreated.Before(rankings[right].DateCreated)
	})

	loopLimit := len(rankings)
	if options.Limit > 0 && options.Limit < loopLimit {
		loopLimit = options.Limit
	}

	body := ""
	for index := range loopLimit {
		ranking := rankings[index]
		body += fmt.Sprintf("%d. %s - %s\n", index+1, ranking.FullName, timeAgo(ranking.DateCreated, options.Now))
	}

	return body
}

func BuildFooter(normalisedRows NormalisedRows, options Options) string {
	footer := fmt.Sprintf("\nTotal messages: %d\n\n", normalisedRows.TotalMessage)
	if options.Reverse {
		footer += "between, 深潛會搵唔到 ho chi is\n"
	}
	footer += fmt.Sprintf("Last Update: %s", options.Now.Format(time.RFC3339))
	return footer
}

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

func displayName(row message.Message) string {
	name := strings.TrimSpace(strings.Join([]string{row.FirstName, row.LastName}, " "))
	if name != "" {
		return name
	}
	if row.Username != "" {
		return row.Username
	}

	return fmt.Sprintf("%d", row.UserID)
}

func timeAgo(dateCreated time.Time, now time.Time) string {
	duration := now.Sub(dateCreated)
	if duration < 0 {
		duration = 0
	}

	switch {
	case duration < 45*time.Second:
		return "a few seconds ago"
	case duration < 90*time.Second:
		return "a minute ago"
	case duration < 45*time.Minute:
		return plural(int(duration.Minutes()+0.5), "minute")
	case duration < 90*time.Minute:
		return "an hour ago"
	case duration < 22*time.Hour:
		return plural(int(duration.Hours()+0.5), "hour")
	case duration < 36*time.Hour:
		return "a day ago"
	case duration < 26*24*time.Hour:
		return plural(int(duration.Hours()/24+0.5), "day")
	case duration < 45*24*time.Hour:
		return "a month ago"
	case duration < 320*24*time.Hour:
		return plural(int(duration.Hours()/(24*30)+0.5), "month")
	case duration < 548*24*time.Hour:
		return "a year ago"
	default:
		return plural(int(duration.Hours()/(24*365)+0.5), "year")
	}
}

func plural(value int, unit string) string {
	if value == 1 {
		if unit == "hour" {
			return "an hour ago"
		}
		return "a " + unit + " ago"
	}

	return fmt.Sprintf("%d %ss ago", value, unit)
}
