// Package workday handles the stored workday bitmask format.
package workday

import (
	"fmt"
	"time"
)

// Workday bit values stored in chat settings.
const (
	Sun = 1
	Mon = 2
	Tue = 4
	Wed = 8
	Thu = 16
	Fri = 32
	Sat = 64
)

const allDaysMask = Sun | Mon | Tue | Wed | Thu | Fri | Sat

// Workdays is the validated stored bitmask for enabled workdays.
type Workdays int

// Parse validates a stored workday bitmask.
func Parse(mask int) (Workdays, error) {
	if mask < 0 || mask&^allDaysMask != 0 {
		return 0, fmt.Errorf("invalid workday mask %d", mask)
	}

	return Workdays(mask), nil
}

// Contains reports whether date falls on an enabled workday.
func Contains(workdays Workdays, date time.Time) bool {
	return int(workdays)&bitForWeekday(date.Weekday()) != 0
}

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
