package dynamodb

import (
	"testing"
	"time"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
)

func TestEncodeDynamoValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input map[string]any
		want  map[string]ddbtypes.AttributeValue
	}{
		{name: "nil input", input: nil, want: nil},
		{
			name:  "bool",
			input: map[string]any{":value": true},
			want: map[string]ddbtypes.AttributeValue{
				":value": &ddbtypes.AttributeValueMemberBOOL{Value: true},
			},
		},
		{
			name:  "float",
			input: map[string]any{":value": 2.5},
			want: map[string]ddbtypes.AttributeValue{
				":value": &ddbtypes.AttributeValueMemberN{Value: "2.5"},
			},
		},
		{
			name:  "int",
			input: map[string]any{":value": 7},
			want: map[string]ddbtypes.AttributeValue{
				":value": &ddbtypes.AttributeValueMemberN{Value: "7"},
			},
		},
		{
			name:  "int64",
			input: map[string]any{":value": int64(42)},
			want: map[string]ddbtypes.AttributeValue{
				":value": &ddbtypes.AttributeValueMemberN{Value: "42"},
			},
		},
		{
			name:  "string",
			input: map[string]any{":value": "ops"},
			want: map[string]ddbtypes.AttributeValue{
				":value": &ddbtypes.AttributeValueMemberS{Value: "ops"},
			},
		},
		{
			name:  "fallback string",
			input: map[string]any{":value": []string{"fallback"}},
			want: map[string]ddbtypes.AttributeValue{
				":value": &ddbtypes.AttributeValueMemberS{Value: "[fallback]"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, encodeDynamoValues(test.input))
		})
	}
}

func TestDecodeMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		item        map[string]ddbtypes.AttributeValue
		want        message.Message
		wantErrText string
	}{
		{
			name: "valid item",
			item: map[string]ddbtypes.AttributeValue{
				"chatId":      &ddbtypes.AttributeValueMemberN{Value: "123"},
				"chatTitle":   &ddbtypes.AttributeValueMemberS{Value: "Ops"},
				"dateCreated": &ddbtypes.AttributeValueMemberS{Value: "2026-05-02T20:30:00+08:00"},
				"firstName":   &ddbtypes.AttributeValueMemberS{Value: "Ada"},
				"lastName":    &ddbtypes.AttributeValueMemberS{Value: "Lovelace"},
				"ttl":         &ddbtypes.AttributeValueMemberN{Value: "456"},
				"userId":      &ddbtypes.AttributeValueMemberN{Value: "789"},
				"username":    &ddbtypes.AttributeValueMemberS{Value: "ada"},
			},
			want: message.Message{
				ChatID:      123,
				ChatTitle:   "Ops",
				DateCreated: time.Date(2026, 5, 2, 20, 30, 0, 0, time.FixedZone("", 8*60*60)),
				FirstName:   "Ada",
				LastName:    "Lovelace",
				TTL:         456,
				UserID:      789,
				Username:    "ada",
			},
		},
		{
			name: "invalid timestamp",
			item: map[string]ddbtypes.AttributeValue{
				"dateCreated": &ddbtypes.AttributeValueMemberS{Value: "bad"},
			},
			wantErrText: "parse dateCreated",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row, err := decodeMessage(test.item)
			if test.wantErrText != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErrText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, row)
		})
	}
}

func TestDecodeChat(t *testing.T) {
	t.Parallel()

	enableAllJung := true
	workday := 62

	row := decodeChat(map[string]ddbtypes.AttributeValue{
		"chatId":        &ddbtypes.AttributeValueMemberN{Value: "123"},
		"chatTitle":     &ddbtypes.AttributeValueMemberS{Value: "Ops"},
		"dateCreated":   &ddbtypes.AttributeValueMemberS{Value: "2026-05-02T20:30:00+08:00"},
		"enableAllJung": &ddbtypes.AttributeValueMemberBOOL{Value: true},
		"offTime":       &ddbtypes.AttributeValueMemberS{Value: "1830"},
		"ttl":           &ddbtypes.AttributeValueMemberN{Value: "456"},
		"workday":       &ddbtypes.AttributeValueMemberN{Value: "62"},
	})

	assert.Equal(t, chat.Row{
		ChatID:        123,
		ChatTitle:     "Ops",
		DateCreated:   "2026-05-02T20:30:00+08:00",
		EnableAllJung: &enableAllJung,
		OffTime:       "1830",
		TTL:           456,
		Workday:       &workday,
	}, row)
}

func TestAttributeHelpersFallbacks(t *testing.T) {
	t.Parallel()

	item := map[string]ddbtypes.AttributeValue{
		"badBool":   &ddbtypes.AttributeValueMemberS{Value: "true"},
		"badInt":    &ddbtypes.AttributeValueMemberS{Value: "6"},
		"badInt64":  &ddbtypes.AttributeValueMemberS{Value: "42"},
		"badString": &ddbtypes.AttributeValueMemberN{Value: "42"},
		"nanInt":    &ddbtypes.AttributeValueMemberN{Value: "NaN"},
		"nanInt64":  &ddbtypes.AttributeValueMemberN{Value: "NaN"},
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{name: "missing string", got: stringAttribute(item, "missing"), want: ""},
		{name: "wrong string type", got: stringAttribute(item, "badString"), want: ""},
		{name: "missing int64", got: int64Attribute(item, "missing"), want: int64(0)},
		{name: "wrong int64 type", got: int64Attribute(item, "badInt64"), want: int64(0)},
		{name: "bad int64 value", got: int64Attribute(item, "nanInt64"), want: int64(0)},
		{name: "missing int", got: intAttribute(item, "missing"), want: (*int)(nil)},
		{name: "wrong int type", got: intAttribute(item, "badInt"), want: (*int)(nil)},
		{name: "bad int value", got: intAttribute(item, "nanInt"), want: (*int)(nil)},
		{name: "missing bool", got: boolAttribute(item, "missing"), want: (*bool)(nil)},
		{name: "wrong bool type", got: boolAttribute(item, "badBool"), want: (*bool)(nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.got)
		})
	}
}

func TestDecodeLastEvaluatedKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item map[string]ddbtypes.AttributeValue
		want map[string]any
	}{
		{
			name: "empty",
			item: nil,
			want: nil,
		},
		{
			name: "known types",
			item: map[string]ddbtypes.AttributeValue{
				"bool":        &ddbtypes.AttributeValueMemberBOOL{Value: true},
				"number":      &ddbtypes.AttributeValueMemberN{Value: "42"},
				"rawNumber":   &ddbtypes.AttributeValueMemberN{Value: "cursor"},
				"string":      &ddbtypes.AttributeValueMemberS{Value: "value"},
				"unsupported": &ddbtypes.AttributeValueMemberNULL{Value: true},
			},
			want: map[string]any{
				"bool":      true,
				"number":    int64(42),
				"rawNumber": "cursor",
				"string":    "value",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, decodeLastEvaluatedKey(test.item))
		})
	}
}
