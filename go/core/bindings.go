package core

// #include "rust/core.h"
import "C"
import (
	"fmt"
	"unsafe"
)

// ActionResult represents the response from Rust command processing.
// This mirrors the C struct ActionResult from the FFI layer.
type ActionResult struct {
	StatusCode   int
	ChatID       int64
	ResponseText string
}

// ProcessWebhook wraps the Rust process_webhook function.
//
// It accepts a Telegram webhook payload (JSON string), passes it to Rust
// for processing, and returns the result containing:
// - StatusCode: HTTP status code (200 for success, 500 for errors)
// - ChatID: Telegram chat ID where the response should be sent
// - ResponseText: Formatted message text to send back to Telegram
//
// The function handles all memory management internally, including freeing
// C strings allocated by Rust.
func ProcessWebhook(payload string) (ActionResult, error) {
	// Convert Go string to C string
	cPayload := C.CString(payload)
	defer C.free(unsafe.Pointer(cPayload))

	// Call Rust FFI function
	cResult := C.process_webhook(cPayload)

	// Extract response text and free it
	var responseText string
	if cResult.response_text != nil {
		responseText = C.GoString(cResult.response_text)
		C.free_string(cResult.response_text)
	}

	result := ActionResult{
		StatusCode:   int(cResult.status_code),
		ChatID:       int64(cResult.chat_id),
		ResponseText: responseText,
	}

	// Check for error status codes
	if result.StatusCode >= 400 {
		return result, fmt.Errorf("rust processing failed with status code %d", result.StatusCode)
	}

	return result, nil
}
