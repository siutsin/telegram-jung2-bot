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

func TestContainsChecksDateWeekday(t *testing.T) {
	workdays, err := Parse(Mon | Wed | Fri)
	require.NoError(t, err)

	tests := []struct {
		name string
		date string
		want bool
	}{
		{name: "monday", date: "2026-04-27T00:00:00Z", want: true},
		{name: "tuesday", date: "2026-04-28T00:00:00Z", want: false},
		{name: "wednesday", date: "2026-04-29T00:00:00Z", want: true},
		{name: "saturday", date: "2026-05-02T00:00:00Z", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			date, err := time.Parse(time.RFC3339, test.date)
			require.NoError(t, err)

			assert.Equal(t, test.want, Contains(workdays, date))
		})
	}
}
