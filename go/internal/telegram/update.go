// Package telegram contains Telegram transport models and helpers.
package telegram

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ReportLimit is the project safety limit below Telegram's 4096 character cap.
const ReportLimit = 3800

// Update is the subset of Telegram updates used by this service.
type Update struct {
	Message *Message `json:"message"`
}

// Message is the subset of Telegram messages used by this service.
type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
	From *User  `json:"from,omitempty"`
}

// Chat identifies a Telegram chat.
type Chat struct {
	ID    int64  `json:"id"`
	Title string `json:"title,omitempty"`
	Type  string `json:"type,omitempty"`
}

// User identifies a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	UserName  string `json:"username,omitempty"`
}

// Administrator identifies a Telegram chat administrator.
type Administrator struct {
	User User `json:"user"`
}

// ParseUpdate decodes a Telegram webhook update.
func ParseUpdate(payload []byte) (Update, error) {
	if strings.TrimSpace(string(payload)) == "" {
		return Update{}, fmt.Errorf("telegram update payload is empty")
	}

	var update Update
	if err := json.Unmarshal(payload, &update); err != nil {
		return Update{}, fmt.Errorf("decode telegram update: %w", err)
	}

	return update, nil
}

// TruncateReport trims text to the project report limit without splitting UTF-8
// runes.
func TruncateReport(text string) string {
	if utf8.RuneCountInString(text) <= ReportLimit {
		return text
	}

	runes := []rune(text)
	return string(runes[:ReportLimit])
}
