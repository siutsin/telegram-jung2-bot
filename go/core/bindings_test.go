package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessWebhook verifies that the FFI binding correctly calls Rust
// and returns a properly formatted response.
func TestProcessWebhook(t *testing.T) {
	// Valid payload (currently returns placeholder response)
	payload := `{"message":{"chat":{"id":123},"text":"test"}}`

	result, err := ProcessWebhook(payload)
	require.NoError(t, err, "ProcessWebhook should not return an error")

	assert.Equal(t, 200, result.StatusCode, "expected status code 200")
	assert.NotEmpty(t, result.ResponseText, "expected non-empty response text")

	t.Logf("Response: %+v", result)
}

// TestProcessWebhookEmptyPayload verifies that empty payloads are handled gracefully.
func TestProcessWebhookEmptyPayload(t *testing.T) {
	payload := ""

	result, err := ProcessWebhook(payload)
	require.NoError(t, err, "ProcessWebhook should not return an error")

	// Even with empty payload, Rust should return a response
	// (currently the placeholder implementation returns 200 with a message)
	assert.Equal(t, 200, result.StatusCode, "expected status code 200 for empty payload")
}

// TestProcessWebhookInvalidJSON tests handling of malformed JSON.
func TestProcessWebhookInvalidJSON(t *testing.T) {
	payload := `{invalid json`

	result, err := ProcessWebhook(payload)
	require.NoError(t, err, "ProcessWebhook should not return an error")

	// Currently returns placeholder response; when real parsing is implemented,
	// this should return an appropriate error status
	t.Logf("Invalid JSON resulted in: %+v", result)
}
