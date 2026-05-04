package message

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

func TestFromTelegram(t *testing.T) {
	now := mustParseTime(t)
	tests := []struct {
		name  string
		input telegram.Message
		want  Message
	}{
		{
			name: "full legacy message",
			input: telegram.Message{
				Chat: telegram.Chat{ID: -4, Title: "title"},
				From: &telegram.User{ID: 3, UserName: "username", FirstName: "first_name", LastName: "last_name"},
			},
			want: Message{
				ChatID:      -4,
				DateCreated: now.In(storageLocation),
				ChatTitle:   "title",
				UserID:      3,
				Username:    "username",
				FirstName:   "first_name",
				LastName:    "last_name",
				TTL:         now.Add(DefaultTTL).Unix(),
			},
		},
		{
			name: "legacy optional user fields",
			input: telegram.Message{
				Chat: telegram.Chat{ID: -4},
				From: &telegram.User{},
			},
			want: Message{
				ChatID:      -4,
				DateCreated: now.In(storageLocation),
				TTL:         now.Add(DefaultTTL).Unix(),
			},
		},
		{
			name: "missing user",
			input: telegram.Message{
				Chat: telegram.Chat{ID: -4},
			},
			want: Message{
				ChatID:      -4,
				DateCreated: now.In(storageLocation),
				TTL:         now.Add(DefaultTTL).Unix(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, FromTelegram(test.input, now))
		})
	}
}

func TestDateCreatedContract(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		want        string
		wantErrText string
	}{
		{
			name: "round trip contract UTC plus eight",
			raw:  "2019-03-16T02:26:19+08:00",
			want: "2019-03-16T02:26:19+08:00",
		},
		{
			name:        "invalid value",
			raw:         "not-a-date",
			wantErrText: "parse dateCreated",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ParseDateCreated(test.raw)
			if test.wantErrText != "" {
				require.ErrorContains(t, err, test.wantErrText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, FormatDateCreated(parsed))
		})
	}
}

func TestTTLUsesContractSevenDayRetention(t *testing.T) {
	now := mustParseTime(t)

	assert.Equal(t, int64(1554691104), TTL(now, DefaultTTL))
}

func mustParseTime(t *testing.T) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, "2019-04-01T02:38:24Z")
	require.NoError(t, err)

	return parsed
}
