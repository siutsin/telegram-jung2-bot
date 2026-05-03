// Package workday handles the stored workday bitmask format.
package workday

import (
	"fmt"
	"strings"
	"time"
)

// Workday bit values stored in chat settings.
const (
	Sun = 1 << iota
	Mon
	Tue
	Wed
	Thu
	Fri
	Sat
)

const allDaysMask = Sun | Mon | Tue | Wed | Thu | Fri | Sat

// Workdays is the validated stored bitmask for enabled workdays.
type Workdays int

var dayBits = map[string]int{
	"SUN": Sun,
	"MON": Mon,
	"TUE": Tue,
	"WED": Wed,
	"THU": Thu,
	"FRI": Fri,
	"SAT": Sat,
}

// Parse validates a stored workday bitmask.
// For example, 6 becomes MON|TUE, while 128 is rejected.
func Parse(mask int) (Workdays, error) {
	if mask < 0 || mask&^allDaysMask != 0 {
		return 0, fmt.Errorf("invalid workday mask %d", mask)
	}

	return Workdays(mask), nil
}

// ParseList converts the contract comma-separated day list into a workday mask.
// For example, "MON,TUE" becomes 6.
func ParseList(raw string) (Workdays, error) {
	if raw == "" {
		return 0, fmt.Errorf("workday list is required")
	}

	var mask int
	for day := range strings.SplitSeq(raw, ",") {
		bit, ok := dayBits[day]
		if !ok {
			return 0, fmt.Errorf("invalid workday %q", day)
		}
		mask |= bit
	}

	return Parse(mask)
}

// Contains reports whether date falls on an enabled workday.
// For example, MON|TUE contains a Monday timestamp.
func Contains(workdays Workdays, date time.Time) bool {
	return int(workdays)&bitForWeekday(date.Weekday()) != 0
}

// MatchesDay reports whether a contract day token is enabled in the workday mask.
// For example, "MON" matches MON|TUE, while "SUN" does not.
func MatchesDay(day string, workdays Workdays) bool {
	bit, ok := dayBits[day]
	if !ok {
		return false
	}

	return int(workdays)&bit != 0
}

// bitForWeekday returns the stored bit for a weekday.
// For example, time.Monday becomes Mon.
func bitForWeekday(weekday time.Weekday) int {
	switch weekday {
	case time.Sunday:
		return Sun
	case time.Monday:
		return Mon
	case time.Tuesday:
		return Tue
	case time.Wednesday:
		return Wed
	case time.Thursday:
		return Thu
	case time.Friday:
		return Fri
	case time.Saturday:
		return Sat
	default:
		return 0
	}
}
