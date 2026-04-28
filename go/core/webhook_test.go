package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessWebhook verifies that the webhook scaffold decodes a Telegram
// message and returns the chat ID needed by later command handling.
func TestProcessWebhook(t *testing.T) {
	payload := `{"message":{"chat":{"id":123},"text":"test"}}`

	result, err := ProcessWebhook(payload)
	require.NoError(t, err, "ProcessWebhook should not return an error")

	assert.Equal(t, 200, result.StatusCode, "expected status code 200")
	assert.Equal(t, int64(123), result.ChatID, "expected chat ID from payload")
	assert.NotEmpty(t, result.ResponseText, "expected non-empty response text")
}

// TestProcessWebhookEmptyPayload verifies that empty payloads are handled gracefully.
func TestProcessWebhookEmptyPayload(t *testing.T) {
	payload := ""

	result, err := ProcessWebhook(payload)
	require.Error(t, err, "ProcessWebhook should reject an empty payload")

	assert.Equal(t, 400, result.StatusCode, "expected status code 400 for empty payload")
}

// TestProcessWebhookInvalidJSON tests handling of malformed JSON.
func TestProcessWebhookInvalidJSON(t *testing.T) {
	payload := `{invalid json`

	result, err := ProcessWebhook(payload)
	require.Error(t, err, "ProcessWebhook should reject malformed JSON")

	assert.Equal(t, 400, result.StatusCode, "expected status code 400 for malformed JSON")
}
