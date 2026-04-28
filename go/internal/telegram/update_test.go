package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUpdateDecodesSupportedMessage(t *testing.T) {
	payload := []byte(`{"message":{"chat":{"id":123,"title":"Test","type":"group"},"from":{"id":456,"first_name":"Ada","username":"ada"},"text":"/topTen"}}`)

	update, err := ParseUpdate(payload)
	require.NoError(t, err)
	require.NotNil(t, update.Message)

	assert.Equal(t, int64(123), update.Message.Chat.ID)
	assert.Equal(t, "Test", update.Message.Chat.Title)
	assert.Equal(t, "/topTen", update.Message.Text)
	require.NotNil(t, update.Message.From)
	assert.Equal(t, int64(456), update.Message.From.ID)
}

func TestParseUpdateAcceptsUnsupportedUpdate(t *testing.T) {
	update, err := ParseUpdate([]byte(`{"edited_message":{"text":"ignored"}}`))
	require.NoError(t, err)

	assert.Nil(t, update.Message)
}

func TestParseUpdateRejectsEmptyPayload(t *testing.T) {
	_, err := ParseUpdate([]byte(" "))
	require.Error(t, err)
}

func TestParseUpdateRejectsMalformedJSON(t *testing.T) {
	_, err := ParseUpdate([]byte(`{invalid json`))
	require.Error(t, err)
}

func TestTruncateReportKeepsShortText(t *testing.T) {
	assert.Equal(t, "hello", TruncateReport("hello"))
}

func TestTruncateReportKeepsValidUTF8(t *testing.T) {
	text := strings.Repeat("冗", ReportLimit+10)

	truncated := TruncateReport(text)

	assert.Equal(t, ReportLimit, utf8.RuneCountInString(truncated))
	assert.True(t, utf8.ValidString(truncated))
}
