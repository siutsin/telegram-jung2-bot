package workday

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAcceptsValidMasks(t *testing.T) {
	tests := []struct {
		name string
		mask int
	}{
		{name: "empty", mask: 0},
		{name: "sunday", mask: Sun},
		{name: "weekdays", mask: Mon | Tue | Wed | Thu | Fri},
		{name: "all days", mask: Sun | Mon | Tue | Wed | Thu | Fri | Sat},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := Parse(test.mask)
			require.NoError(t, err)
			assert.Equal(t, Workdays(test.mask), actual)
		})
	}
}

func TestParseRejectsInvalidMasks(t *testing.T) {
	tests := []int{-1, 128, 255}

	for _, mask := range tests {
		t.Run("mask", func(t *testing.T) {
			_, err := Parse(mask)
			require.Error(t, err)
		})
	}
}

func TestWeekdayMaskIsSixtyTwo(t *testing.T) {
	assert.Equal(t, 62, Mon|Tue|Wed|Thu|Fri)
}

func TestParseListMatchesContractWorkdayStringToBinary(t *testing.T) {
	tests := []struct {
		raw  string
		want int
	}{
		{raw: "MON,TUE,WED,THU,FRI", want: 62},
		{raw: "MON,TUE,WED,THU", want: 30},
		{raw: "SUN,MON,TUE,WED,THU", want: 31},
		{raw: "MON,MON", want: Mon},
	}

	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			workdays, err := ParseList(test.raw)
			require.NoError(t, err)
			assert.Equal(t, Workdays(test.want), workdays)
		})
	}
}

func TestParseListRejectsInvalidContractWorkdayString(t *testing.T) {
	tests := []string{"", "mon", "MON,", "MON,FRIDAY", " MON"}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseList(raw)
			require.Error(t, err)
		})
	}
}

func TestMatchesDayMatchesContractIsWeekdayMatchBinary(t *testing.T) {
	tests := []struct {
		name     string
		workday  int
		trueDays []string
		falseDay []string
	}{
		{
			name:     "MON to FRI",
			workday:  62,
			trueDays: []string{"MON", "TUE", "WED", "THU", "FRI"},
			falseDay: []string{"SAT", "SUN"},
		},
		{
			name:     "SUN to THU",
			workday:  31,
			trueDays: []string{"SUN", "MON", "TUE", "WED", "THU"},
			falseDay: []string{"FRI", "SAT"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workdays, err := Parse(test.workday)
			require.NoError(t, err)

			for _, day := range test.trueDays {
				assert.True(t, MatchesDay(day, workdays), day)
			}
			for _, day := range test.falseDay {
				assert.False(t, MatchesDay(day, workdays), day)
			}
			assert.False(t, MatchesDay("HOLIDAY", workdays))
		})
	}
}

func TestContainsChecksDateWeekday(t *testing.T) {
	workdays, err := Parse(Sun | Mon | Tue | Wed | Thu | Fri | Sat)
	require.NoError(t, err)

	tests := []struct {
		name string
		date string
		want bool
	}{
		{name: "sunday", date: "2026-04-26T00:00:00Z", want: true},
		{name: "monday", date: "2026-04-27T00:00:00Z", want: true},
		{name: "tuesday", date: "2026-04-28T00:00:00Z", want: true},
		{name: "wednesday", date: "2026-04-29T00:00:00Z", want: true},
		{name: "thursday", date: "2026-04-30T00:00:00Z", want: true},
		{name: "friday", date: "2026-05-01T00:00:00Z", want: true},
		{name: "saturday", date: "2026-05-02T00:00:00Z", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			date, err := time.Parse(time.RFC3339, test.date)
			require.NoError(t, err)

			assert.Equal(t, test.want, Contains(workdays, date))
		})
	}
}

func TestContainsRejectsDateOutsideMask(t *testing.T) {
	workdays, err := Parse(Mon)
	require.NoError(t, err)

	date, err := time.Parse(time.RFC3339, "2026-04-28T00:00:00Z")
	require.NoError(t, err)

	assert.False(t, Contains(workdays, date))
}

func TestBitForWeekdayRejectsUnknownWeekday(t *testing.T) {
	assert.Equal(t, 0, bitForWeekday(time.Weekday(99)))
}
