// Package telegram contains Telegram transport models and helpers.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// ReportLimit is the project safety limit below Telegram's 4096 character cap.
const ReportLimit = 3800

const defaultAPIBaseURL = "https://api.telegram.org"

// Client calls the Telegram Bot API.
type Client struct {
	baseURL    string
	botToken   string
	httpClient *http.Client
}

type SendMessageOptions struct {
	DisableWebPagePreview bool
	ParseMode             string
}

// ClientOption customises a Telegram client.
type ClientOption func(*Client)

// WithBaseURL overrides the Telegram API base URL for tests or local proxies.
func WithBaseURL(baseURL string) ClientOption {
	return func(client *Client) {
		client.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

// NewClient creates a Telegram Bot API client.
func NewClient(botToken string, options ...ClientOption) Client {
	client := Client{
		baseURL:    defaultAPIBaseURL,
		botToken:   botToken,
		httpClient: http.DefaultClient,
	}

	for _, option := range options {
		option(&client)
	}

	return client
}

// Update is the subset of Telegram updates used by this service.
type Update struct {
	Message *Message `json:"message"`
}

// Message is the subset of Telegram messages used by this service.
type Message struct {
	Text     string   `json:"text"`
	Chat     Chat     `json:"chat"`
	From     *User    `json:"from,omitempty"`
	Entities []Entity `json:"entities,omitempty"`
}

type Entity struct {
	Type string `json:"type"`
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

// SendMessage sends text to a Telegram chat.
func (client Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return client.SendMessageWithOptions(ctx, chatID, text, SendMessageOptions{})
}

// SendMessageWithOptions sends text to a Telegram chat with optional Telegram
// API sendMessage fields.
func (client Client) SendMessageWithOptions(ctx context.Context, chatID int64, text string, options SendMessageOptions) (err error) {
	requestBody := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if options.DisableWebPagePreview {
		requestBody["disable_web_page_preview"] = true
	}
	if options.ParseMode != "" {
		requestBody["parse_mode"] = options.ParseMode
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshal sendMessage request: %w", err)
	}

	response, err := client.do(ctx, http.MethodPost, "sendMessage", nil, payload)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, closeResponseBody(response.Body, "sendMessage"))
	}()

	err = telegramAPIError(response)
	if err != nil {
		return err
	}

	return nil
}

// GetChatAdministrators returns the administrators of a Telegram chat.
func (client Client) GetChatAdministrators(ctx context.Context, chatID int64) (administrators []Administrator, err error) {
	query := url.Values{"chat_id": {fmt.Sprint(chatID)}}
	response, err := client.do(ctx, http.MethodGet, "getChatAdministrators", query, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, closeResponseBody(response.Body, "getChatAdministrators"))
	}()

	err = telegramAPIError(response)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result []Administrator `json:"result"`
	}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("decode getChatAdministrators response: %w", err)
	}

	return result.Result, nil
}

// IsAdmin reports whether userID is an administrator of chatID.
func (client Client) IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error) {
	administrators, err := client.GetChatAdministrators(ctx, chatID)
	if err != nil {
		return false, err
	}

	for _, administrator := range administrators {
		if administrator.User.ID == userID {
			return true, nil
		}
	}

	return false, nil
}

// ParseUpdate decodes a Telegram webhook update.
func ParseUpdate(payload []byte) (Update, error) {
	if strings.TrimSpace(string(payload)) == "" {
		return Update{}, fmt.Errorf("telegram update payload is empty")
	}

	var update Update
	err := json.Unmarshal(payload, &update)
	if err != nil {
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

// do sends a Telegram API request.
func (client Client) do(
	ctx context.Context,
	method string,
	endpoint string,
	query url.Values,
	payload []byte,
) (*http.Response, error) {
	requestURL := fmt.Sprintf("%s/bot%s/%s", client.baseURL, client.botToken, endpoint)
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	request, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create Telegram %s request: %w", endpoint, err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Telegram %s: %w", endpoint, err)
	}

	return response, nil
}

// telegramAPIError converts non-2xx responses into errors.
func telegramAPIError(response *http.Response) error {
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	statusErr := fmt.Errorf("telegram API returned HTTP %d", response.StatusCode)
	_, err := io.Copy(io.Discard, response.Body)
	if err != nil {
		return errors.Join(statusErr, fmt.Errorf("drain telegram API error response: %w", err))
	}

	return statusErr
}

// closeResponseBody closes a Telegram HTTP response body with context.
func closeResponseBody(body io.Closer, endpoint string) error {
	err := body.Close()
	if err != nil {
		return fmt.Errorf("close Telegram %s response body: %w", endpoint, err)
	}

	return nil
}
